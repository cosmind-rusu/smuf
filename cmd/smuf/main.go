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
	flag.Parse()

	if *showHelp {
		printClientHelp()
		return
	}
	if *showVersion {
		fmt.Println("smuf v0.2.0")
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
		fmt.Println()
	}

	// Obtener el puerto
	var port string
	if flag.NArg() >= 1 {
		port = flag.Arg(0)
	} else {
		// Preguntar por el puerto si no se dio
		if isInteractive() {
			fmt.Print("  Puerto de tu app local: ")
			fmt.Scanln(&port)
		}
		if port == "" {
			fmt.Println("Uso: smuf <puerto>")
			fmt.Println("Ejemplo: smuf 3000")
			os.Exit(1)
		}
	}

	serverAddr := envOr("SMUF_SERVER", defaultServer)
	authToken := os.Getenv("SMUF_AUTH_TOKEN")

	conn, err := dialWithRetry(serverAddr, 5, 2*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError: cannot reach smuf-server at %s\n", serverAddr)
		fmt.Fprintf(os.Stderr, "Tip: set SMUF_SERVER=host:port if using a custom server\n")
		os.Exit(1)
	}

	// --- Handshake ---
	if authToken != "" {
		fmt.Fprintf(conn, "AUTH %s PORT %s\n", authToken, port)
	} else {
		fmt.Fprintf(conn, "PORT %s\n", port)
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: handshake failed: %v\n", err)
		os.Exit(1)
	}

	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "OK ") {
		fmt.Fprintf(os.Stderr, "Error: server rejected: %s\n", line)
		os.Exit(1)
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		fmt.Fprintf(os.Stderr, "Error: invalid server response: %s\n", line)
		os.Exit(1)
	}

	id := fields[1]
	publicURL := ""
	if len(fields) >= 3 {
		publicURL = fields[2]
	}

	// --- Upgrade a yamux ---
	// Usamos BufConn para que los bytes ya bufferizados del handshake no se pierdan
	session, err := yamux.Client(tunnel.NewBufConn(conn, reader), yamux.DefaultConfig())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: yamux failed: %v\n", err)
		os.Exit(1)
	}
	defer session.Close()

	if publicURL == "" {
		serverHost := strings.Split(serverAddr, ":")[0]
		httpPort := envOr("SMUF_HTTP_PORT", "8080")
		publicURL = fmt.Sprintf("http://%s.%s:%s", id, serverHost, httpPort)
	}

	printBanner(port, publicURL)

	// Aceptamos streams que el servidor nos envía (uno por petición HTTP entrante)
	go serveStreams(session, port)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nTunnel closed.")
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
  smuf --setup             Configurar conexión al servidor
  smuf -h                  Mostrar esta ayuda

Ejemplos:
  smuf 3000                Exponer localhost:3000
  smuf 8080                Exponer localhost:8080

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

func printBanner(port, publicURL string) {
	fmt.Println()
	fmt.Println("  Tunnel ready!")
	fmt.Println()
	fmt.Printf("  Local   → http://localhost:%s\n", port)
	fmt.Printf("  Public  → %s\n", publicURL)
	fmt.Println()
	fmt.Println("  Press Ctrl+C to stop")
	fmt.Println()
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
