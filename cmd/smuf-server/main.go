package main

import (
	"bufio"
	"crypto/rand"
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
)

func main() {
	controlPort := envOr("SMUF_CONTROL_PORT", "7000")
	httpPort := envOr("SMUF_HTTP_PORT", "8080")
	domain := envOr("SMUF_DOMAIN", "localhost")

	registry := tunnel.NewRegistry()

	go startHTTP(httpPort, domain, registry)
	startControl(controlPort, httpPort, domain, registry)
}

// startControl acepta conexiones TCP de clientes smuf y las registra como túneles.
func startControl(port, httpPort, domain string, registry *tunnel.Registry) {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal("cannot bind control port %s: %v", port, err)
	}
	defer ln.Close()

	logger.Info("smuf-server ready | control :%s | http :%s | domain %s", port, httpPort, domain)

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("accept: %v", err)
			return
		}
		go handleTunnel(conn, httpPort, domain, registry)
	}
}

// handleTunnel ejecuta el handshake, establece la sesión yamux y bloquea
// hasta que el cliente desconecte.
func handleTunnel(conn net.Conn, httpPort, domain string, registry *tunnel.Registry) {
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

	// Respondemos con el ID asignado
	fmt.Fprintf(conn, "OK %s\n", id)

	// Hacemos upgrade a yamux usando BufConn para no perder bytes ya bufferizados
	session, err := yamux.Server(tunnel.NewBufConn(conn, reader), yamux.DefaultConfig())
	if err != nil {
		logger.Error("[%s] yamux init: %v", id, err)
		conn.Close()
		return
	}

	registry.Add(id, session)
	logger.Info("[%s] tunnel open → local port %s | public http://%s.%s:%s", id, localPort, id, domain, httpPort)

	defer func() {
		session.Close()
		registry.Remove(id)
		logger.Info("[%s] tunnel closed", id)
	}()

	// Bloqueamos hasta que la sesión yamux termine (cliente desconecta o error)
	<-session.CloseChan()
}

// startHTTP levanta el servidor HTTP público que enruta peticiones a túneles activos.
func startHTTP(port, domain string, registry *tunnel.Registry) {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logger.Fatal("cannot bind http port %s: %v", port, err)
	}
	logger.Info("http proxy listening on :%s", port)

	srv := &http.Server{Handler: &httpProxy{domain: domain, registry: registry}}
	srv.Serve(ln)
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
