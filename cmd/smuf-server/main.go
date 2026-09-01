package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cdrusu/smuf/internal/logger"
	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/cdrusu/smuf/internal/wizard"
	"github.com/hashicorp/yamux"
	"golang.org/x/crypto/acme/autocert"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
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
		fmt.Println("smuf-server " + version)
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

	tcpRange := envOr("SMUF_TCP_PORT_RANGE", "")
	tcpStart, tcpEnd := 0, 0
	if tcpRange != "" {
		parts := strings.Split(tcpRange, "-")
		if len(parts) == 2 {
			tcpStart, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
			tcpEnd, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
		}
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
		connRatePerMin:   envInt("SMUF_CONN_RATE_PER_MIN", 30),
		handshakeTimeout: envDuration("SMUF_HANDSHAKE_TIMEOUT", 10*time.Second),
		tcpEnabled:       tcpRange != "" && tcpStart > 0 && tcpEnd >= tcpStart,
		tcpPortStart:     tcpStart,
		tcpPortEnd:       tcpEnd,
	}

	registry := tunnel.NewRegistry()
	rateLimiter := newIPRateLimiter(cfg.maxConnsPerIP)
	connRateLimiter := newConnRateAttemptLimiter(cfg.connRatePerMin, time.Minute)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			connRateLimiter.sweep()
		}
	}()
	var tcpAllocator *tcpPortAllocator
	if cfg.tcpEnabled {
		tcpAllocator = newTCPPortAllocator(cfg.tcpPortStart, cfg.tcpPortEnd)
	}

	ln, err := net.Listen("tcp", ":"+cfg.controlPort)
	if err != nil {
		logger.Fatal("cannot bind control port %s: %v", cfg.controlPort, err)
	}

	go startPublicHTTP(cfg, registry)

	var wg sync.WaitGroup
	go func() {
		startControl(ln, cfg, registry, rateLimiter, connRateLimiter, tcpAllocator, &wg)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	ln.Close()
	// Cerrar las sesiones yamux activas desbloquea los handleTunnel, que
	// hacen su limpieza (registry, puertos TCP) y dejan avanzar a wg.
	registry.CloseAll()
	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		logger.Error("shutdown timeout: forzando salida con túneles aún activos")
	}
	logger.Info("goodbye")
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
  SMUF_MAX_CONNS_PER_IP    Límite de túneles concurrentes por IP (default: 5)
  SMUF_CONN_RATE_PER_MIN   Intentos de conexión por IP por minuto (default: 30)

La configuración se guarda automáticamente en tu perfil de usuario al usar --setup.`)
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
	connRatePerMin   int
	handshakeTimeout time.Duration
	// TCP puro
	tcpEnabled       bool
	tcpPortStart     int
	tcpPortEnd       int
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

// connRateAttemptLimiter limita cuántos *intentos* de conexión nuevos acepta
// por IP en una ventana de tiempo, independientemente de cuántas estén
// activas a la vez. ipRateLimiter (arriba) sólo limita conexiones
// concurrentes, así que no frena a un cliente que abre y cierra conexiones
// rápidamente en ráfaga.
type connRateAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newConnRateAttemptLimiter(limit int, window time.Duration) *connRateAttemptLimiter {
	return &connRateAttemptLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (r *connRateAttemptLimiter) allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-r.window)

	kept := r.attempts[ip][:0]
	for _, t := range r.attempts[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= r.limit {
		r.attempts[ip] = kept
		return false
	}
	r.attempts[ip] = append(kept, now)
	return true
}

// sweep elimina IPs sin intentos recientes para no crecer sin límite en
// despliegues de larga duración con muchas IPs distintas.
func (r *connRateAttemptLimiter) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-r.window)
	for ip, times := range r.attempts {
		if len(times) == 0 || times[len(times)-1].Before(cutoff) {
			delete(r.attempts, ip)
		}
	}
}

// tcpPortAllocator gestiona un rango de puertos TCP públicos para túneles puros.
type tcpPortAllocator struct {
	mu       sync.Mutex
	start    int
	end      int
	assigned map[string]int // tunnel ID -> puerto
	inUse    map[int]string // puerto -> tunnel ID
}

func newTCPPortAllocator(start, end int) *tcpPortAllocator {
	return &tcpPortAllocator{
		start:    start,
		end:      end,
		assigned: make(map[string]int),
		inUse:    make(map[int]string),
	}
}

func (a *tcpPortAllocator) allocate(id string) (int, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.assigned[id]; ok {
		return p, true
	}
	for p := a.start; p <= a.end; p++ {
		if _, used := a.inUse[p]; !used {
			if ln, err := net.Listen("tcp", fmt.Sprintf(":%d", p)); err == nil {
				ln.Close()
				a.assigned[id] = p
				a.inUse[p] = id
				return p, true
			}
		}
	}
	return 0, false
}

func (a *tcpPortAllocator) release(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.assigned[id]; ok {
		delete(a.inUse, p)
		delete(a.assigned, id)
	}
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
func startControl(ln net.Listener, cfg serverConfig, registry *tunnel.Registry, rateLimiter *ipRateLimiter, connRateLimiter *connRateAttemptLimiter, tcpAllocator *tcpPortAllocator, wg *sync.WaitGroup) {
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
	if cfg.tcpEnabled {
		logger.Info("tcp tunnel range :%d-%d", cfg.tcpPortStart, cfg.tcpPortEnd)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return // apagado: listener cerrado desde main
			}
			logger.Error("accept: %v", err)
			return
		}

		// Rate limiting por IP: primero la tasa de intentos (ráfagas), luego el
		// tope de conexiones concurrentes.
		ip := extractIP(conn.RemoteAddr().String())
		if !connRateLimiter.allow(ip) {
			logger.Error("connection rate limit exceeded for IP %s", ip)
			conn.Close()
			continue
		}
		if !rateLimiter.acquire(ip) {
			logger.Error("rate limit exceeded for IP %s", ip)
			conn.Close()
			continue
		}

		wg.Add(1)
		go func(c net.Conn, clientIP string) {
			defer wg.Done()
			defer rateLimiter.release(clientIP)
			handleTunnel(c, cfg, registry, tcpAllocator)
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
func handleTunnel(conn net.Conn, cfg serverConfig, registry *tunnel.Registry, tcpAllocator *tcpPortAllocator) {
	// Timeout para el handshake completo
	conn.SetReadDeadline(time.Now().Add(cfg.handshakeTimeout))

	limitedReader := io.LimitReader(conn, 1024) // Límite de 1KB para handshake
	bufReader := bufio.NewReader(limitedReader)

	// Handshake: esperamos "PORT <puerto>\n", "TCP <puerto>\n" o "AUTH <token> PORT/TCP <puerto>\n"
	line, err := bufReader.ReadString('\n')
	if err != nil {
		logger.Error("handshake from %s: %v", conn.RemoteAddr(), err)
		conn.Close()
		return
	}

	parts := strings.Fields(strings.TrimSpace(line))

	// Autenticación requerida si SMUF_AUTH_TOKEN está configurado
	cmdIdx := 0
	if cfg.authToken != "" {
		if len(parts) < 4 || parts[0] != "AUTH" {
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
		cmdIdx = 2
	}

	if len(parts) < cmdIdx+2 {
		fmt.Fprintf(conn, "ERR invalid handshake\n")
		conn.Close()
		return
	}

	cmd := parts[cmdIdx]
	localPort := parts[cmdIdx+1]
	portNum, err := strconv.Atoi(localPort)
	if err != nil || portNum < 1 || portNum > 65535 {
		fmt.Fprintf(conn, "ERR invalid port\n")
		conn.Close()
		return
	}

	isTCP := cmd == "TCP"
	isHTTP := cmd == "PORT"
	if !isTCP && !isHTTP {
		fmt.Fprintf(conn, "ERR invalid handshake\n")
		conn.Close()
		return
	}

	// Subdominio personalizado solo para HTTP
	requestedSub := ""
	if isHTTP && len(parts) == cmdIdx+4 && parts[cmdIdx+2] == "SUB" {
		requestedSub = strings.ToLower(parts[cmdIdx+3])
		if !isValidSubdomain(requestedSub) {
			fmt.Fprintf(conn, "ERR invalid subdomain: solo letras, números y guiones (1-63 chars)\n")
			conn.Close()
			return
		}
		if registry.Has(requestedSub) {
			fmt.Fprintf(conn, "ERR subdomain in use\n")
			conn.Close()
			return
		}
	}

	// Limpiar deadline después del handshake exitoso
	conn.SetReadDeadline(time.Time{})

	var id string
	if requestedSub != "" {
		id = requestedSub
	} else {
		id, err = newUniqueID(registry)
		if err != nil {
			logger.Error("failed to generate tunnel ID: %v", err)
			fmt.Fprintf(conn, "ERR internal error\n")
			conn.Close()
			return
		}
	}

	var publicURL string
	var publicTCPPort int
	if isTCP {
		if !cfg.tcpEnabled {
			fmt.Fprintf(conn, "ERR tcp tunnels disabled\n")
			conn.Close()
			return
		}
		var ok bool
		publicTCPPort, ok = tcpAllocator.allocate(id)
		if !ok {
			fmt.Fprintf(conn, "ERR no tcp ports available\n")
			conn.Close()
			return
		}
		publicURL = fmt.Sprintf("tcp://%s:%d", cfg.domain, publicTCPPort)
	} else {
		publicURL = publicTunnelURL(id, cfg)
	}

	// Hacemos upgrade a yamux usando BufConn para no perder bytes ya bufferizados.
	// OJO: el LimitReader del handshake NO puede seguir en la cadena de lectura
	// de yamux: al agotar sus 1024 bytes devolvería EOF permanente y la sesión
	// moriría tras ~1KB de tráfico. Drenamos primero lo bufferizado y luego
	// leemos del socket directamente.
	session, err := yamux.Server(tunnel.NewBufConn(conn, io.MultiReader(bufReader, conn)), yamux.DefaultConfig())
	if err != nil {
		logger.Error("[%s] yamux init: %v", id, err)
		if isTCP {
			tcpAllocator.release(id)
		}
		conn.Close()
		return
	}

	entry := &tunnel.TunnelEntry{
		Session:   session,
		Type:      tunnel.TunnelHTTP,
		Port:      localPort,
		PublicURL: publicURL,
		ClientIP:  extractIP(conn.RemoteAddr().String()),
		CreatedAt: time.Now(),
	}
	if isTCP {
		entry.Type = tunnel.TunnelTCP
		entry.PublicTCPPort = strconv.Itoa(publicTCPPort)
	}

	// Registro atómico: el Has() de arriba da el error amable en el caso común,
	// pero entre Has() y este punto otro cliente pudo ganar el mismo subdominio.
	if !registry.AddIfAbsent(id, entry) {
		logger.Error("[%s] tunnel ID conflict (race) from %s", id, conn.RemoteAddr())
		fmt.Fprintf(conn, "ERR subdomain in use\n")
		session.Close()
		if isTCP {
			tcpAllocator.release(id)
		}
		return
	}

	// Respondemos con el ID asignado y la URL pública que debe mostrar el cliente.
	fmt.Fprintf(conn, "OK %s %s\n", id, publicURL)
	logger.Info("[%s] tunnel open → type=%s local port %s | public %s", id, entry.Type, localPort, publicURL)

	// Para TCP, arrancar listener público
	var tcpListener net.Listener
	if isTCP {
		tcpListener, err = net.Listen("tcp", fmt.Sprintf(":%d", publicTCPPort))
		if err != nil {
			logger.Error("[%s] cannot bind tcp port %d: %v", id, publicTCPPort, err)
			session.Close()
			registry.Remove(id)
			tcpAllocator.release(id)
			return
		}
		go serveTCPListener(id, tcpListener, session)
	}

	defer func() {
		if tcpListener != nil {
			tcpListener.Close()
			tcpAllocator.release(id)
		}
		session.Close()
		registry.Remove(id)
		logger.Info("[%s] tunnel closed", id)
	}()

	// Bloqueamos hasta que la sesión yamux termine (cliente desconecta o error)
	<-session.CloseChan()
}

// serveTCPListener acepta conexiones TCP en el puerto público y las reenvía
// a streams yamux del túnel correspondiente.
func serveTCPListener(tunnelID string, ln net.Listener, session *yamux.Session) {
	logger.Info("[%s] tcp listener started on %s", tunnelID, ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			return // listener cerrado
		}
		go func(c net.Conn) {
			defer c.Close()
			stream, err := session.Open()
			if err != nil {
				logger.Error("[%s] open yamux stream: %v", tunnelID, err)
				return
			}
			defer stream.Close()
			// Copia bidireccional de bytes crudos
			done := make(chan struct{}, 2)
			go func() { io.Copy(stream, c); done <- struct{}{} }()
			go func() { io.Copy(c, stream); done <- struct{}{} }()
			<-done
		}(conn)
	}
}

// startPublicHTTP levanta el servidor público. En modo HTTP enruta peticiones
// directamente; en modo HTTPS atiende challenges ACME y redirige el resto.
func startPublicHTTP(cfg serverConfig, registry *tunnel.Registry) {
	proxy := newHTTPProxy(cfg.domain, registry)

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
		Handler: proxy,
		// Sólo limitamos el tiempo de las cabeceras. Un ReadTimeout o un
		// WriteTimeout globales cortarían a mitad subidas grandes, descargas
		// lentas y respuestas en streaming (SSE, long-polling).
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
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
		Addr:              ":" + cfg.httpPort,
		Handler:           manager.HTTPHandler(redirectToHTTPS(cfg)),
		ReadHeaderTimeout: 30 * time.Second,
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
		// Ver nota en startPublicHTTP: nada de ReadTimeout/WriteTimeout.
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
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

// tunnelCtxKey identifica el túnel resuelto para una petición. ServeHTTP lo
// deja en el contexto y el RoundTripper lo recupera, de modo que el
// ReverseProxy (que sólo ve la petición) sabe por qué sesión yamux enviarla.
type tunnelCtxKey struct{}

type tunnelTarget struct {
	id    string
	entry *tunnel.TunnelEntry
}

type httpProxy struct {
	domain   string
	registry *tunnel.Registry
	rp       *httputil.ReverseProxy

	mu sync.Mutex
	// Un http.Transport por sesión yamux: así el pool de conexiones nunca
	// mezcla streams de una sesión vieja con un túnel reconectado que
	// reutiliza el mismo subdominio.
	transports map[*yamux.Session]*http.Transport
}

func newHTTPProxy(domain string, registry *tunnel.Registry) *httpProxy {
	p := &httpProxy{
		domain:     domain,
		registry:   registry,
		transports: make(map[*yamux.Session]*http.Transport),
	}
	p.rp = &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme = "http"
			// Mantenemos el Host público (abc123.tudominio.com) para que la
			// app local vea exactamente lo mismo que antes de este cambio.
			pr.Out.URL.Host = pr.In.Host
			// Limpia las X-Forwarded-* que llegasen de fuera y pone las
			// nuestras: sin X-Forwarded-Proto, una app detrás de HTTPS
			// genera redirects a http:// y rompe cualquier login.
			pr.SetXForwarded()
		},
		Transport: p,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			id := ""
			if t, ok := r.Context().Value(tunnelCtxKey{}).(tunnelTarget); ok {
				id = t.id
			}
			logger.Error("[%s] proxy %s %s: %v", id, r.Method, r.URL.Path, err)
			http.Error(w, "tunnel unavailable", http.StatusBadGateway)
		},
		// El ErrorHandler ya registra el fallo; evitamos el log duplicado.
		ErrorLog: log.New(io.Discard, "", 0),
	}
	return p
}

// RoundTrip envía la petición por la sesión yamux del túnel que ServeHTTP
// dejó en el contexto.
func (p *httpProxy) RoundTrip(r *http.Request) (*http.Response, error) {
	target, ok := r.Context().Value(tunnelCtxKey{}).(tunnelTarget)
	if !ok {
		return nil, fmt.Errorf("no tunnel in request context")
	}
	return p.transportFor(target.entry.Session).RoundTrip(r)
}

// transportFor devuelve (creándolo si hace falta) el transport asociado a una
// sesión yamux. Cada "conexión" que abre el transport es un stream nuevo.
func (p *httpProxy) transportFor(sess *yamux.Session) *http.Transport {
	p.mu.Lock()
	if t, ok := p.transports[sess]; ok {
		p.mu.Unlock()
		return t
	}
	t := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return sess.Open()
		},
		// El cliente final decide su Accept-Encoding; no comprimimos ni
		// descomprimimos por nuestra cuenta.
		DisableCompression:    true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       60 * time.Second,
		ExpectContinueTimeout: time.Second,
		// Sin ResponseHeaderTimeout a propósito: una app local legítimamente
		// lenta no debe ver su respuesta cortada.
	}
	p.transports[sess] = t
	p.mu.Unlock()

	go func() {
		<-sess.CloseChan()
		p.mu.Lock()
		delete(p.transports, sess)
		p.mu.Unlock()
		t.CloseIdleConnections()
	}()
	return t
}

func (p *httpProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Extraemos el ID del subdominio: "abc123.localhost" → "abc123"
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Dashboard: petición al dominio raíz (no a un subdominio de túnel)
	if host == p.domain {
		p.serveDashboard(w, r)
		return
	}

	suffix := "." + p.domain
	if !strings.HasSuffix(host, suffix) {
		http.Error(w, "bad request: missing tunnel subdomain", http.StatusBadRequest)
		return
	}
	id := strings.TrimSuffix(host, suffix)

	entry, ok := p.registry.Get(id)
	if !ok {
		http.Error(w, "tunnel unavailable", http.StatusNotFound)
		return
	}

	// CONNECT no lo cubre ReverseProxy: seguimos tunelizando bytes crudos.
	if r.Method == http.MethodConnect {
		stream, err := entry.Session.Open()
		if err != nil {
			logger.Error("[%s] open stream: %v", id, err)
			http.Error(w, "tunnel unavailable", http.StatusBadGateway)
			return
		}
		defer stream.Close()
		handleUpgrade(w, r, stream)
		return
	}

	// ReverseProxy se encarga del resto: WebSockets y otros upgrades (101),
	// respuestas en streaming (hace Flush inmediato en SSE y en respuestas
	// sin Content-Length) y la limpieza de cabeceras hop-by-hop.
	ctx := context.WithValue(r.Context(), tunnelCtxKey{}, tunnelTarget{id: id, entry: entry})
	p.rp.ServeHTTP(w, r.WithContext(ctx))
}

func (p *httpProxy) serveDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/_smuf/tunnels" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(p.registry.List())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, dashboardHTML)
}

const dashboardHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>smuf · dashboard</title>
<style>
  :root{
    --bg-deep:#0d0e12;
    --bg-surface:#15181e;
    --bg-card:#17191f;
    --border:#26292f;
    --text-primary:#efeff1;
    --text-secondary:#d5d7db;
    --text-muted:#656a76;
    --text-faint:#3b3d45;
    --accent:#1060ff;
    --accent-hover:#2b89ff;
    --accent-danger:#e53e3e;
    --success:#22c55e;
    --shadow:rgba(97,104,117,0.05) 0px 1px 1px, rgba(97,104,117,0.05) 0px 2px 2px;
  }
  *{box-sizing:border-box;margin:0;padding:0}
  body{
    background:var(--bg-deep);
    color:var(--text-primary);
    font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",Helvetica,Arial,sans-serif;
    font-size:16px;
    line-height:1.63;
    min-height:100vh;
    -webkit-font-smoothing:antialiased;
  }
  .wrap{max-width:1152px;margin:0 auto;padding:64px 24px}
  /* Header */
  .header{display:flex;align-items:baseline;gap:16px;margin-bottom:56px}
  .logo{
    font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
    font-size:28px;font-weight:700;line-height:1.19;letter-spacing:-0.02em;
    color:var(--text-primary);
  }
  .sub{
    font-size:13px;font-weight:600;line-height:1.69;
    text-transform:uppercase;letter-spacing:1.3px;
    color:var(--text-muted);
  }
  /* Section label */
  .section-label{
    display:flex;align-items:center;justify-content:space-between;
    margin-bottom:16px;
  }
  .section-label span{
    font-size:13px;font-weight:600;line-height:1.69;
    text-transform:uppercase;letter-spacing:1.3px;
    color:var(--text-muted);
  }
  .count{font-size:13px;color:var(--text-faint);font-weight:500}
  /* Cards */
  .card-grid{display:flex;flex-direction:column;gap:12px}
  .card{
    background:var(--bg-card);
    border:1px solid var(--border);
    border-radius:8px;
    box-shadow:var(--shadow);
    padding:20px 24px;
    display:flex;align-items:flex-start;justify-content:space-between;gap:24px;
    transition:border-color .2s ease;
  }
  .card:hover{border-color:rgba(97,104,117,0.25)}
  .card-main{flex:1;min-width:0}
  .card-meta{display:flex;align-items:center;gap:10px;margin-bottom:10px;flex-wrap:wrap}
  .dot{width:8px;height:8px;border-radius:50%;background:var(--success);display:inline-block;flex-shrink:0}
  .badge{
    font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:0.8px;
    padding:2px 7px;border-radius:5px;
    background:rgba(16,96,255,0.12);color:var(--accent-hover);
    border:1px solid rgba(16,96,255,0.25);
  }
  .badge.tcp{background:rgba(123,66,188,0.12);color:#b388ff;border-color:rgba(123,66,188,0.25)}
  .idcode{font-size:12px;color:var(--text-muted);font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace}
  .ago{font-size:12px;color:var(--text-faint)}
  .url{
    font-size:15px;color:var(--accent);text-decoration:none;
    word-break:break-all;font-weight:500;
  }
  .url:hover{color:var(--accent-hover);text-decoration:underline}
  .side{text-align:right;font-size:13px;color:var(--text-muted);flex-shrink:0;line-height:1.6}
  .side strong{color:var(--text-secondary);font-weight:600}
  /* Empty / error */
  .empty{color:var(--text-faint);font-size:15px;padding:24px 0}
  .error{color:var(--accent-danger);font-size:15px;padding:24px 0}
  /* Footer */
  .footer{margin-top:56px;display:flex;align-items:center;gap:16px;font-size:13px;color:var(--text-faint)}
  .footer a{color:var(--text-muted);text-decoration:none;font-weight:500}
  .footer a:hover{color:var(--text-secondary);text-decoration:underline}
  /* Responsive */
  @media (max-width:600px){
    .wrap{padding:40px 16px}
    .header{margin-bottom:36px}
    .logo{font-size:22px}
    .card{padding:16px;gap:16px;flex-direction:column}
    .side{text-align:left}
  }
</style>
</head>
<body>
<div class="wrap">
  <div class="header">
    <span class="logo">smuf</span>
    <span class="sub">dashboard</span>
  </div>
  <div class="section-label">
    <span>Túneles activos</span>
    <span class="count" id="count"></span>
  </div>
  <div id="tunnels" class="card-grid">
    <p class="empty">Cargando...</p>
  </div>
  <div class="footer">
    <span>Actualiza cada 5 s</span>
    <span>·</span>
    <a href="/_smuf/tunnels">JSON API</a>
  </div>
</div>
<script>
function ago(iso){
  var s=Math.floor((Date.now()-new Date(iso))/1000);
  if(s<60)return s+'s';
  if(s<3600)return Math.floor(s/60)+'m';
  var h=Math.floor(s/3600);
  return h+'h '+Math.floor((s%3600)/60)+'m';
}
function render(tunnels){
  var count=document.getElementById('count');
  var container=document.getElementById('tunnels');
  if(!tunnels||tunnels.length===0){
    count.textContent='';
    container.innerHTML='<p class="empty">No hay túneles activos.</p>';
    return;
  }
  count.textContent=tunnels.length+' activo'+(tunnels.length===1?'':'s');
  var html='';
  for(var i=0;i<tunnels.length;i++){
    var t=tunnels[i];
    var isTCP=t.type==='tcp';
    var badgeClass=isTCP?'badge tcp':'badge';
    var badgeText=isTCP?'TCP':'HTTP';
    html+='<div class="card">';
    html+='<div class="card-main">';
    html+='<div class="card-meta">';
    html+='<span class="dot"></span>';
    html+='<span class="'+badgeClass+'">'+badgeText+'</span>';
    html+='<span class="idcode">'+t.id+'</span>';
    html+='<span class="ago">hace '+ago(t.created_at)+'</span>';
    html+='</div>';
    html+='<a href="'+t.public_url+'" target="_blank" rel="noopener" class="url">'+t.public_url+'</a>';
    html+='</div>';
    html+='<div class="side">';
    html+='<div>local <strong>:'+t.port+'</strong></div>';
    html+='<div>'+t.client_ip+'</div>';
    if(t.public_tcp_port){
      html+='<div>tcp <strong>:'+t.public_tcp_port+'</strong></div>';
    }
    html+='</div>';
    html+='</div>';
  }
  container.innerHTML=html;
}
function refresh(){
  fetch('/_smuf/tunnels')
    .then(function(r){return r.json();})
    .then(render)
    .catch(function(){
      document.getElementById('tunnels').innerHTML='<p class="error">Error al contactar el servidor.</p>';
    });
}
refresh();
setInterval(refresh,5000);
</script>
</body>
</html>`

func isValidSubdomain(s string) bool {
	if len(s) < 1 || len(s) > 63 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	return true
}

func newID() (string, error) {
	b := make([]byte, 6) // 48 bits = 12 hex chars: colisión insignificante incluso con miles de túneles activos
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// newUniqueID genera un ID y lo verifica contra el registry para descartar
// la (muy improbable) colisión, evitando que un ID nuevo pise silenciosamente
// una entrada existente en el registry.
func newUniqueID(registry *tunnel.Registry) (string, error) {
	const maxAttempts = 5
	for i := 0; i < maxAttempts; i++ {
		id, err := newID()
		if err != nil {
			return "", err
		}
		if !registry.Has(id) {
			return id, nil
		}
	}
	return "", fmt.Errorf("no se pudo generar un ID único tras %d intentos", maxAttempts)
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

// handleUpgrade toma el control de la conexión TCP del cliente (hijack)
// y pasa a modo túnel de bytes entre el cliente y el stream yamux.
// Sólo se usa para CONNECT: los upgrades HTTP normales (WebSocket) los
// gestiona ReverseProxy.
func handleUpgrade(w http.ResponseWriter, r *http.Request, stream net.Conn) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer conn.Close()
	defer stream.Close()

	// Reconstruir y enviar la petición HTTP original por el stream yamux
	if err := r.Write(stream); err != nil {
		logger.Error("upgrade forward: %v", err)
		return
	}

	// Si el bufio.Reader del hijack tiene bytes ya leídos (por ejemplo,
	// el inicio de un frame WebSocket enviado inmediatamente tras la petición),
	// los enviamos primero para no perderlos.
	if bufrw.Reader.Buffered() > 0 {
		data, _ := bufrw.Reader.Peek(bufrw.Reader.Buffered())
		if _, err := stream.Write(data); err != nil {
			logger.Error("upgrade buffered write: %v", err)
			return
		}
	}

	// Copia bidireccional de bytes hasta que un lado cierre
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(stream, conn)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(conn, stream)
		errChan <- err
	}()
	<-errChan
}
