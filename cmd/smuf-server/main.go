package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/cdrusu/smuf/internal/logger"
	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/hashicorp/yamux"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	cfg := serverConfig{
		controlPort:    envOr("SMUF_CONTROL_PORT", "7000"),
		httpPort:       envOr("SMUF_HTTP_PORT", "8080"),
		httpsPort:      envOr("SMUF_HTTPS_PORT", "443"),
		domain:         envOr("SMUF_DOMAIN", "localhost"),
		httpsEnabled:   envBool("SMUF_HTTPS", false),
		acmeCacheDir:   envOr("SMUF_ACME_CACHE", "certs"),
		acmeEmail:      os.Getenv("SMUF_ACME_EMAIL"),
		publicHTTPPort: envOr("SMUF_PUBLIC_HTTP_PORT", ""),
		publicTLSPort:  envOr("SMUF_PUBLIC_HTTPS_PORT", ""),
	}

	registry := tunnel.NewRegistry()

	go startPublicHTTP(cfg, registry)
	startControl(cfg, registry)
}

type serverConfig struct {
	controlPort    string
	httpPort       string
	httpsPort      string
	domain         string
	httpsEnabled   bool
	acmeCacheDir   string
	acmeEmail      string
	publicHTTPPort string
	publicTLSPort  string
}

// startControl acepta conexiones TCP de clientes smuf y las registra como túneles.
func startControl(cfg serverConfig, registry *tunnel.Registry) {
	ln, err := net.Listen("tcp", ":"+cfg.controlPort)
	if err != nil {
		logger.Fatal("cannot bind control port %s: %v", cfg.controlPort, err)
	}
	defer ln.Close()

	if cfg.httpsEnabled {
		logger.Info("smuf-server ready | control :%s | http :%s | https :%s | domain %s", cfg.controlPort, cfg.httpPort, cfg.httpsPort, cfg.domain)
	} else {
		logger.Info("smuf-server ready | control :%s | http :%s | domain %s", cfg.controlPort, cfg.httpPort, cfg.domain)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("accept: %v", err)
			return
		}
		go handleTunnel(conn, cfg, registry)
	}
}

// handleTunnel ejecuta el handshake, establece la sesión yamux y bloquea
// hasta que el cliente desconecte.
func handleTunnel(conn net.Conn, cfg serverConfig, registry *tunnel.Registry) {
	reader := bufio.NewReader(conn)

	// Handshake: esperamos "PORT <puerto>\n"
	line, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("handshake from %s: %v", conn.RemoteAddr(), err)
		conn.Close()
		return
	}

	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) != 2 || parts[0] != "PORT" {
		fmt.Fprintf(conn, "ERR invalid handshake\n")
		conn.Close()
		return
	}

	localPort := parts[1]
	id := newID()
	publicURL := publicTunnelURL(id, cfg)

	// Respondemos con el ID asignado y la URL pública que debe mostrar el cliente.
	fmt.Fprintf(conn, "OK %s %s\n", id, publicURL)

	// Hacemos upgrade a yamux usando BufConn para no perder bytes ya bufferizados
	session, err := yamux.Server(tunnel.NewBufConn(conn, reader), yamux.DefaultConfig())
	if err != nil {
		logger.Error("[%s] yamux init: %v", id, err)
		conn.Close()
		return
	}

	registry.Add(id, session)
	logger.Info("[%s] tunnel open → local port %s | public %s", id, localPort, publicURL)

	defer func() {
		session.Close()
		registry.Remove(id)
		logger.Info("[%s] tunnel closed", id)
	}()

	// Bloqueamos hasta que la sesión yamux termine (cliente desconecta o error)
	<-session.CloseChan()
}

// startPublicHTTP levanta el servidor público. En modo HTTP enruta peticiones
// directamente; en modo HTTPS atiende challenges ACME y redirige el resto.
func startPublicHTTP(cfg serverConfig, registry *tunnel.Registry) {
	proxy := &httpProxy{domain: cfg.domain, registry: registry}

	if cfg.httpsEnabled {
		startHTTPS(cfg, proxy)
		return
	}

	ln, err := net.Listen("tcp", ":"+cfg.httpPort)
	if err != nil {
		logger.Fatal("cannot bind http port %s: %v", cfg.httpPort, err)
	}
	logger.Info("http proxy listening on :%s", cfg.httpPort)

	srv := &http.Server{Handler: proxy}
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		logger.Fatal("http server: %v", err)
	}
}

func startHTTPS(cfg serverConfig, proxy http.Handler) {
	manager := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cfg.acmeCacheDir),
		Email:      cfg.acmeEmail,
		HostPolicy: tunnelHostPolicy(cfg.domain),
	}

	httpSrv := &http.Server{
		Addr:    ":" + cfg.httpPort,
		Handler: manager.HTTPHandler(redirectToHTTPS(cfg)),
	}
	go func() {
		logger.Info("http ACME/redirect listening on :%s", cfg.httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("http ACME/redirect server: %v", err)
		}
	}()

	tlsSrv := &http.Server{
		Addr:      ":" + cfg.httpsPort,
		Handler:   proxy,
		TLSConfig: &tls.Config{GetCertificate: manager.GetCertificate, MinVersion: tls.VersionTLS12},
	}

	logger.Info("https proxy listening on :%s | acme cache %s", cfg.httpsPort, cfg.acmeCacheDir)
	if err := tlsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		logger.Fatal("https server: %v", err)
	}
}

func tunnelHostPolicy(domain string) autocert.HostPolicy {
	return func(_ context.Context, host string) error {
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.TrimSuffix(strings.ToLower(host), ".")
		domain = strings.TrimSuffix(strings.ToLower(domain), ".")
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return nil
		}
		return fmt.Errorf("acme host not allowed: %s", host)
	}
}

func redirectToHTTPS(cfg serverConfig) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		targetHost := withPort(host, publicPort(cfg.publicTLSPort, cfg.httpsPort))
		http.Redirect(w, r, "https://"+targetHost+r.URL.RequestURI(), http.StatusMovedPermanently)
	})
}

type httpProxy struct {
	domain   string
	registry *tunnel.Registry
}

func (p *httpProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extraemos el ID del subdominio: "abc123.localhost" → "abc123"
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	suffix := "." + p.domain
	if !strings.HasSuffix(host, suffix) {
		http.Error(w, "bad request: missing tunnel subdomain", http.StatusBadRequest)
		return
	}
	id := strings.TrimSuffix(host, suffix)

	session, ok := p.registry.Get(id)
	if !ok {
		http.Error(w, "tunnel not found: "+id, http.StatusNotFound)
		return
	}

	// Abrimos un stream yamux para esta petición concreta
	stream, err := session.Open()
	if err != nil {
		http.Error(w, "tunnel unavailable", http.StatusBadGateway)
		logger.Error("[%s] open stream: %v", id, err)
		return
	}
	defer stream.Close()

	// Escribimos la petición HTTP en el stream (el cliente la reenvía a localhost)
	if err := r.Write(stream); err != nil {
		http.Error(w, "forward error", http.StatusBadGateway)
		return
	}

	// Leemos la respuesta que el cliente nos devuelve por el mismo stream
	resp, err := http.ReadResponse(bufio.NewReader(stream), r)
	if err != nil {
		http.Error(w, "response error", http.StatusBadGateway)
		logger.Error("[%s] read response: %v", id, err)
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func newID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func publicTunnelURL(id string, cfg serverConfig) string {
	host := id + "." + cfg.domain
	if cfg.httpsEnabled {
		port := publicPort(cfg.publicTLSPort, cfg.httpsPort)
		return "https://" + withPort(host, port)
	}
	port := publicPort(cfg.publicHTTPPort, cfg.httpPort)
	return "http://" + withPort(host, port)
}

func publicPort(publicPort, listenPort string) string {
	if publicPort != "" {
		return publicPort
	}
	return listenPort
}

func withPort(host, port string) string {
	if port == "" || port == "80" || port == "443" {
		return host
	}
	return net.JoinHostPort(host, port)
}
