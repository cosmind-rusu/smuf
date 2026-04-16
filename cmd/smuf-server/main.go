package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cdrusu/smuf/internal/logger"
	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/cdrusu/smuf/internal/wizard"
	"github.com/hashicorp/yamux"
	"golang.org/x/crypto/acme/autocert"
)

func main() {
	// Flags para modo no-interactivo
	showHelp := flag.Bool("h", false, "Mostrar ayuda")
	showVersion := flag.Bool("v", false, "Mostrar versión")
	setupMode := flag.Bool("setup", false, "Ejecutar wizard de configuración")
	flag.Parse()

	if *showHelp {
		printServerHelp()
		return
	}
	if *showVersion {
		fmt.Println("smuf-server v0.2.0")
		return
	}

	// Cargar .env si existe
	wizard.LoadEnvFile()

	// Detectar si necesitamos wizard
	needsSetup := *setupMode || (os.Getenv("SMUF_DOMAIN") == "" && isInteractive())

	if needsSetup {
		wizCfg, err := wizard.RunServerWizard()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			os.Exit(1)
		}
		// Aplicar configuración del wizard
		if wizCfg.Domain != "" {
			os.Setenv("SMUF_DOMAIN", wizCfg.Domain)
		}
		if wizCfg.AuthToken != "" {
			os.Setenv("SMUF_AUTH_TOKEN", wizCfg.AuthToken)
		}
		if wizCfg.HTTPPort != "" {
			os.Setenv("SMUF_HTTP_PORT", wizCfg.HTTPPort)
		}
		if wizCfg.HTTPSEnabled {
			os.Setenv("SMUF_HTTPS", "true")
			os.Setenv("SMUF_HTTPS_PORT", wizCfg.HTTPSPort)
			if wizCfg.ACMEEmail != "" {
				os.Setenv("SMUF_ACME_EMAIL", wizCfg.ACMEEmail)
			}
		}
		fmt.Println()
	}

	cfg := serverConfig{
		controlPort:      envOr("SMUF_CONTROL_PORT", "7000"),
		httpPort:         envOr("SMUF_HTTP_PORT", "8080"),
		httpsPort:        envOr("SMUF_HTTPS_PORT", "443"),
		domain:           envOr("SMUF_DOMAIN", "localhost"),
		httpsEnabled:     envBool("SMUF_HTTPS", false),
		acmeCacheDir:     envOr("SMUF_ACME_CACHE", "certs"),
		acmeEmail:        os.Getenv("SMUF_ACME_EMAIL"),
		publicHTTPPort:   envOr("SMUF_PUBLIC_HTTP_PORT", ""),
		publicTLSPort:    envOr("SMUF_PUBLIC_HTTPS_PORT", ""),
		authToken:        os.Getenv("SMUF_AUTH_TOKEN"),
		maxConnsPerIP:    envInt("SMUF_MAX_CONNS_PER_IP", 5),
		handshakeTimeout: envDuration("SMUF_HANDSHAKE_TIMEOUT", 10*time.Second),
	}

	registry := tunnel.NewRegistry()
	rateLimiter := newIPRateLimiter(cfg.maxConnsPerIP)

	go startPublicHTTP(cfg, registry)
	startControl(cfg, registry, rateLimiter)
}

func printServerHelp() {
	fmt.Println(`smuf-server - Servidor de túneles HTTP

Uso:
  smuf-server              Inicia el servidor (wizard si no hay config)
  smuf-server --setup      Forzar wizard de configuración
  smuf-server -h           Mostrar esta ayuda

Variables de entorno:
  SMUF_DOMAIN              Dominio base (ej: tudominio.com)
  SMUF_AUTH_TOKEN          Token de autenticación
  SMUF_HTTP_PORT           Puerto HTTP (default: 8080)
  SMUF_HTTPS               Activar HTTPS (true/false)
  SMUF_HTTPS_PORT          Puerto HTTPS (default: 443)
  SMUF_ACME_EMAIL          Email para Let's Encrypt
  SMUF_CONTROL_PORT        Puerto de control (default: 7000)
  SMUF_MAX_CONNS_PER_IP    Límite de túneles por IP (default: 5)

Puedes crear un archivo .env junto al ejecutable con estas variables.`)
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

type serverConfig struct {
	controlPort      string
	httpPort         string
	httpsPort        string
	domain           string
	httpsEnabled     bool
	acmeCacheDir     string
	acmeEmail        string
	publicHTTPPort   string
	publicTLSPort    string
	authToken        string
	maxConnsPerIP    int
	handshakeTimeout time.Duration
}

// ipRateLimiter controla el número de conexiones activas por IP
type ipRateLimiter struct {
	mu       sync.Mutex
	conns    map[string]int
	maxConns int
}

func newIPRateLimiter(maxConns int) *ipRateLimiter {
	return &ipRateLimiter{
		conns:    make(map[string]int),
		maxConns: maxConns,
	}
}

func (r *ipRateLimiter) acquire(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[ip] >= r.maxConns {
		return false
	}
	r.conns[ip]++
	return true
}

func (r *ipRateLimiter) release(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conns[ip] > 0 {
		r.conns[ip]--
	}
	if r.conns[ip] == 0 {
		delete(r.conns, ip)
	}
}

// startControl acepta conexiones TCP de clientes smuf y las registra como túneles.
func startControl(cfg serverConfig, registry *tunnel.Registry, rateLimiter *ipRateLimiter) {
	ln, err := net.Listen("tcp", ":"+cfg.controlPort)
	if err != nil {
		logger.Fatal("cannot bind control port %s: %v", cfg.controlPort, err)
	}
	defer ln.Close()

	authStatus := "disabled"
	if cfg.authToken != "" {
		authStatus = "enabled"
	}

	if cfg.httpsEnabled {
		logger.Info("smuf-server ready | control :%s | http :%s | https :%s | domain %s | auth %s", cfg.controlPort, cfg.httpPort, cfg.httpsPort, cfg.domain, authStatus)
	} else {
		logger.Info("smuf-server ready | control :%s | http :%s | domain %s | auth %s", cfg.controlPort, cfg.httpPort, cfg.domain, authStatus)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("accept: %v", err)
			return
		}

		// Rate limiting por IP
		ip := extractIP(conn.RemoteAddr().String())
		if !rateLimiter.acquire(ip) {
			logger.Error("rate limit exceeded for IP %s", ip)
			conn.Close()
			continue
		}

		go func(c net.Conn, clientIP string) {
			defer rateLimiter.release(clientIP)
			handleTunnel(c, cfg, registry)
		}(conn, ip)
	}
}

func extractIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// handleTunnel ejecuta el handshake, establece la sesión yamux y bloquea
// hasta que el cliente desconecte.
func handleTunnel(conn net.Conn, cfg serverConfig, registry *tunnel.Registry) {
	// Timeout para el handshake completo
	conn.SetReadDeadline(time.Now().Add(cfg.handshakeTimeout))

	limitedReader := io.LimitReader(conn, 1024) // Límite de 1KB para handshake
	bufReader := bufio.NewReader(limitedReader)

	// Handshake: esperamos "PORT <puerto>\n" o "AUTH <token> PORT <puerto>\n"
	line, err := bufReader.ReadString('\n')
	if err != nil {
		logger.Error("handshake from %s: %v", conn.RemoteAddr(), err)
		conn.Close()
		return
	}

	parts := strings.Fields(strings.TrimSpace(line))

	// Autenticación requerida si SMUF_AUTH_TOKEN está configurado
	if cfg.authToken != "" {
		if len(parts) < 4 || parts[0] != "AUTH" || parts[2] != "PORT" {
			fmt.Fprintf(conn, "ERR authentication required\n")
			conn.Close()
			return
		}
		clientToken := parts[1]
		if subtle.ConstantTimeCompare([]byte(clientToken), []byte(cfg.authToken)) != 1 {
			logger.Error("invalid auth token from %s", conn.RemoteAddr())
			fmt.Fprintf(conn, "ERR invalid token\n")
			conn.Close()
			return
		}
		parts = parts[2:] // Quitar AUTH <token>, dejar PORT <puerto>
	}

	if len(parts) != 2 || parts[0] != "PORT" {
		fmt.Fprintf(conn, "ERR invalid handshake\n")
		conn.Close()
		return
	}

	// Validar que el puerto sea un número válido
	localPort := parts[1]
	portNum, err := strconv.Atoi(localPort)
	if err != nil || portNum < 1 || portNum > 65535 {
		fmt.Fprintf(conn, "ERR invalid port\n")
		conn.Close()
		return
	}

	// Limpiar deadline después del handshake exitoso
	conn.SetReadDeadline(time.Time{})

	id, err := newID()
	if err != nil {
		logger.Error("failed to generate tunnel ID: %v", err)
		fmt.Fprintf(conn, "ERR internal error\n")
		conn.Close()
		return
	}
	publicURL := publicTunnelURL(id, cfg)

	// Respondemos con el ID asignado y la URL pública que debe mostrar el cliente.
	fmt.Fprintf(conn, "OK %s %s\n", id, publicURL)

	// Hacemos upgrade a yamux usando BufConn para no perder bytes ya bufferizados
	session, err := yamux.Server(tunnel.NewBufConn(conn, bufio.NewReader(conn)), yamux.DefaultConfig())
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

	srv := &http.Server{
		Handler:      proxy,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
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
		Addr:         ":" + cfg.httpsPort,
		Handler:      proxy,
		TLSConfig:    &tls.Config{GetCertificate: manager.GetCertificate, MinVersion: tls.VersionTLS12},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
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
		http.Error(w, "tunnel unavailable", http.StatusNotFound)
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

func newID() (string, error) {
	b := make([]byte, 16) // 128 bits de entropía
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
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
