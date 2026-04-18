package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/cdrusu/smuf/internal/wizard"
	"github.com/hashicorp/yamux"
)

const defaultServer = "localhost:7000"

func main() {
	// Flags
	showHelp := flag.Bool("h", false, "Mostrar ayuda")
	showVersion := flag.Bool("v", false, "Mostrar versión")
	setupMode := flag.Bool("setup", false, "Ejecutar wizard de configuración")
	subFlag := flag.String("sub", "", "Subdominio fijo (ej: myapp → myapp.tudominio.com)")
	flag.Parse()

	if *showHelp {
		printClientHelp()
		return
	}
	if *showVersion {
		fmt.Println("smuf v0.3.0")
		return
	}

	// Cargar .env si existe
	wizard.LoadEnvFile()

	// Detectar si necesitamos wizard de configuración
	needsSetup := *setupMode || (os.Getenv("SMUF_SERVER") == "" && os.Getenv("SMUF_SERVER") != defaultServer && isInteractive() && flag.NArg() == 0)

	if needsSetup {
		wizCfg, err := wizard.RunClientWizard()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			os.Exit(1)
		}
		if wizCfg.Server != "" {
			os.Setenv("SMUF_SERVER", wizCfg.Server)
		}
		if wizCfg.AuthToken != "" {
			os.Setenv("SMUF_AUTH_TOKEN", wizCfg.AuthToken)
		}
		if wizCfg.Subdomain != "" {
			os.Setenv("SMUF_SUBDOMAIN", wizCfg.Subdomain)
		}
		fmt.Println()
	}

	// Obtener puertos
	var ports []string
	if flag.NArg() >= 1 {
		ports = flag.Args()
	} else {
		if isInteractive() {
			var port string
			fmt.Print("  Puerto de tu app local: ")
			fmt.Scanln(&port)
			if port != "" {
				ports = []string{port}
			}
		}
		if len(ports) == 0 {
			fmt.Println("Uso: smuf <puerto> [puerto2 ...]")
			fmt.Println("Ejemplo: smuf 3000")
			fmt.Println("         smuf 3000 4000 5000")
			os.Exit(1)
		}
	}

	serverAddr := envOr("SMUF_SERVER", defaultServer)
	authToken := os.Getenv("SMUF_AUTH_TOKEN")
	subdomain := *subFlag
	if subdomain == "" {
		subdomain = os.Getenv("SMUF_SUBDOMAIN")
	}

	type tunnelResult struct {
		port      string
		publicURL string
		session   *yamux.Session
		err       error
	}

	results := make([]tunnelResult, len(ports))
	var wg sync.WaitGroup
	for i, p := range ports {
		wg.Add(1)
		go func(idx int, port string) {
			defer wg.Done()
			sub := ""
			if idx == 0 {
				sub = subdomain
			}
			sess, url, err := connectTunnel(serverAddr, authToken, port, sub)
			results[idx] = tunnelResult{port: port, publicURL: url, session: sess, err: err}
		}(i, p)
	}
	wg.Wait()

	fmt.Println()
	anyOK := false
	if len(ports) == 1 {
		r := results[0]
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "  Error: %v\n", r.err)
			os.Exit(1)
		}
		fmt.Println("  Tunnel ready!")
		fmt.Println()
		fmt.Printf("  Local   → http://localhost:%s\n", r.port)
		fmt.Printf("  Public  → %s\n", r.publicURL)
		anyOK = true
	} else {
		fmt.Println("  Tunnels ready!")
		fmt.Println()
		for _, r := range results {
			if r.err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ :%s — %v\n", r.port, r.err)
			} else {
				fmt.Printf("  localhost:%-6s  →  %s\n", r.port, r.publicURL)
				anyOK = true
			}
		}
	}
	if !anyOK {
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()

	for _, r := range results {
		if r.session != nil {
			go serveStreams(r.session, r.port)
		}
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nTunnel closed.")
}

// connectTunnel establece un único túnel al servidor para el puerto dado.
func connectTunnel(serverAddr, authToken, port, subdomain string) (*yamux.Session, string, error) {
	conn, err := dialWithRetry(serverAddr, 5, 2*time.Second)
	if err != nil {
		return nil, "", fmt.Errorf("cannot reach smuf-server at %s", serverAddr)
	}

	switch {
	case authToken != "" && subdomain != "":
		fmt.Fprintf(conn, "AUTH %s PORT %s SUB %s\n", authToken, port, subdomain)
	case authToken != "":
		fmt.Fprintf(conn, "AUTH %s PORT %s\n", authToken, port)
	case subdomain != "":
		fmt.Fprintf(conn, "PORT %s SUB %s\n", port, subdomain)
	default:
		fmt.Fprintf(conn, "PORT %s\n", port)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("handshake failed: %v", err)
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "OK ") {
		conn.Close()
		return nil, "", fmt.Errorf("server rejected: %s", line)
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		conn.Close()
		return nil, "", fmt.Errorf("invalid server response: %s", line)
	}

	id := fields[1]
	publicURL := ""
	if len(fields) >= 3 {
		publicURL = fields[2]
	}

	session, err := yamux.Client(tunnel.NewBufConn(conn, reader), yamux.DefaultConfig())
	if err != nil {
		conn.Close()
		return nil, "", fmt.Errorf("yamux failed: %v", err)
	}

	if publicURL == "" {
		serverHost := strings.Split(serverAddr, ":")[0]
		httpPort := envOr("SMUF_HTTP_PORT", "8080")
		publicURL = fmt.Sprintf("http://%s.%s:%s", id, serverHost, httpPort)
	}

	return session, publicURL, nil
}

// serveStreams acepta yamux streams y los proxea a la app local en segundo plano.
func serveStreams(session *yamux.Session, localPort string) {
	for {
		stream, err := session.Accept()
		if err != nil {
			return // sesión cerrada (Ctrl+C o error de red)
		}
		go proxyToLocal(stream, localPort)
	}
}

// proxyToLocal conecta el stream yamux a la aplicación local y copia
// bytes en ambas direcciones hasta que uno de los dos lados cierra.
func proxyToLocal(stream net.Conn, port string) {
	defer stream.Close()

	local, err := net.DialTimeout("tcp", "localhost:"+port, 5*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nWarning: localhost:%s unreachable — is your app running?\n", port)
		return
	}
	defer local.Close()

	// Copia bidireccional: esperamos a que cualquiera de los dos lados cierre
	done := make(chan struct{}, 2)
	go func() { io.Copy(local, stream); done <- struct{}{} }()
	go func() { io.Copy(stream, local); done <- struct{}{} }()
	<-done
}

func printClientHelp() {
	fmt.Println(`smuf - Cliente de túneles HTTP

Uso:
  smuf <puerto>            Abre un túnel para localhost:<puerto>
  smuf <p1> <p2> ...       Múltiples túneles en un solo comando
  smuf --sub <nombre> <p>  URL fija: nombre.tudominio.com
  smuf --setup             Configurar conexión al servidor
  smuf -h                  Mostrar esta ayuda

Ejemplos:
  smuf 3000                Exponer localhost:3000
  smuf 3000 4000 5000      Tres túneles simultáneos
  smuf --sub miapp 3000    URL fija: miapp.tudominio.com

Variables de entorno:
  SMUF_SERVER              Dirección del servidor (ej: tudominio.com:7000)
  SMUF_AUTH_TOKEN          Token de autenticación

Puedes crear un archivo .env junto al ejecutable con estas variables.`)
}

func isInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func dialWithRetry(addr string, maxAttempts int, delay time.Duration) (net.Conn, error) {
	var (
		conn net.Conn
		err  error
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err = net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			return conn, nil
		}
		if attempt < maxAttempts {
			fmt.Printf("  Connecting... (attempt %d/%d)\n", attempt, maxAttempts)
			time.Sleep(delay)
		}
	}
	return nil, err
}
