package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/hashicorp/yamux"
	"golang.org/x/net/websocket"
)

// Este test de integración verifica que smuf soporta WebSockets.
// Lo ejecutas con: go run ./cmd/test-websocket
//
// Qué hace:
// 1. Levanta un servidor WebSocket en localhost:19999
// 2. Levanta smuf-server en :17000 (control) y :18080 (HTTP)
// 3. Conecta un cliente smuf a :17000 para tunelizar :19999
// 4. Conecta un cliente WebSocket a http://<id>.localhost:18080
// 5. Envía "hola" y espera recibir "echo: hola"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Servidor WebSocket de prueba
	_ = startWSServer(ctx, ":19999")
	fmt.Println("[test] WebSocket server listening on :19999")

	// 2. smuf-server
	registry := tunnel.NewRegistry()
	go startControlServer(ctx, ":17000", registry)
	go startPublicHTTPServer(ctx, ":18080", registry)
	fmt.Println("[test] smuf-server control :17000, http :18080")

	// Esperar a que los servidores arranquen
	time.Sleep(500 * time.Millisecond)

	// 3. Cliente smuf: conecta al servidor y registra el túnel
	session, tunnelID, err := connectClient(":17000", "19999")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] client connect failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[test] tunnel registered: %s\n", tunnelID)

	// Aceptar streams y proxear al WebSocket server
	go func() {
		for {
			stream, err := session.Accept()
			if err != nil {
				return
			}
			go proxyToLocal(stream, "19999")
		}
	}()

	// 4. Cliente WebSocket: conecta al túnel público
	// Usamos 127.0.0.1 para evitar problemas de resolución DNS en test local
	wsURL := "ws://127.0.0.1:18080/ws"
	fmt.Printf("[test] connecting WebSocket client to %s (Host: %s.localhost)\n", wsURL, tunnelID)

	wsConfig, err := websocket.NewConfig(wsURL, "http://localhost:18080")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] websocket config failed: %v\n", err)
		os.Exit(1)
	}
	wsConfig.Header.Set("Host", tunnelID+".localhost")
	ws, err := websocket.DialConfig(wsConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] websocket dial failed: %v\n", err)
		os.Exit(1)
	}
	defer ws.Close()

	// 5. Enviar mensaje y recibir eco
	msg := []byte("hola")
	if _, err := ws.Write(msg); err != nil {
		fmt.Fprintf(os.Stderr, "[test] websocket write failed: %v\n", err)
		os.Exit(1)
	}

	resp := make([]byte, 512)
	n, err := ws.Read(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] websocket read failed: %v\n", err)
		os.Exit(1)
	}

	expected := "echo: hola"
	got := string(resp[:n])
	if got != expected {
		fmt.Fprintf(os.Stderr, "[test] expected %q, got %q\n", expected, got)
		os.Exit(1)
	}

	fmt.Println("[test] ✅ WebSocket test passed!")

	// Cleanup
	ws.Close()
	cancel()
	time.Sleep(200 * time.Millisecond)
}

// ---- WebSocket echo server ----

func startWSServer(ctx context.Context, addr string) *http.Server {
	http.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		var buf [512]byte
		for {
			n, err := ws.Read(buf[:])
			if err != nil {
				return
			}
			msg := string(buf[:n])
			ws.Write([]byte("echo: " + msg))
		}
	}))

	srv := &http.Server{Addr: addr}
	go srv.ListenAndServe()
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
	return srv
}

// ---- smuf-server simplificado (control) ----

func startControlServer(ctx context.Context, addr string, registry *tunnel.Registry) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go handleControlConn(conn, registry)
	}
}

func handleControlConn(conn net.Conn, registry *tunnel.Registry) {
	defer conn.Close()

	// Handshake simplificado: esperamos "PORT <puerto>\n"
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}
	line := string(buf[:n])
	if len(line) < 6 || line[:5] != "PORT " {
		fmt.Fprintf(conn, "ERR invalid handshake\n")
		return
	}
	port := line[5 : len(line)-1] // quitar \n

	id := "testws01"
	fmt.Fprintf(conn, "OK %s http://%s.localhost:18080\n", id, id)

	session, err := yamux.Server(conn, yamux.DefaultConfig())
	if err != nil {
		return
	}
	defer session.Close()

	registry.Add(id, &tunnel.TunnelEntry{
		Session:   session,
		Port:      port,
		PublicURL: fmt.Sprintf("http://%s.localhost:18080", id),
	})
	defer registry.Remove(id)

	<-session.CloseChan()
}

// ---- smuf-server simplificado (HTTP público) ----

func startPublicHTTPServer(ctx context.Context, addr string, registry *tunnel.Registry) {
	proxy := &testProxy{registry: registry}
	srv := &http.Server{Addr: addr, Handler: proxy}
	go srv.ListenAndServe()
	go func() {
		<-ctx.Done()
		srv.Close()
	}()
}

type testProxy struct {
	registry *tunnel.Registry
}

func (p *testProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// En test local, permitir acceso directo por IP
	id := ""
	if strings.HasSuffix(host, ".localhost") {
		id = strings.TrimSuffix(host, ".localhost")
	} else if host == "127.0.0.1" || host == "localhost" {
		id = "testws01" // túnel de test
	}

	if id == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	entry, ok := p.registry.Get(id)
	if !ok {
		http.Error(w, "tunnel unavailable", http.StatusNotFound)
		return
	}

	stream, err := entry.Session.Open()
	if err != nil {
		http.Error(w, "tunnel error", http.StatusBadGateway)
		return
	}
	defer stream.Close()

	if isUpgradeRequest(r) {
		handleUpgrade(w, r, stream)
		return
	}

	if err := r.Write(stream); err != nil {
		http.Error(w, "forward error", http.StatusBadGateway)
		return
	}
	resp, err := http.ReadResponse(bufio.NewReader(stream), r)
	if err != nil {
		http.Error(w, "response error", http.StatusBadGateway)
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

// ---- Cliente smuf simplificado ----

func connectClient(serverAddr, localPort string) (*yamux.Session, string, error) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, "", err
	}
	fmt.Fprintf(conn, "PORT %s\n", localPort)

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, "", err
	}
	fields := strings.Fields(string(buf[:n]))
	if len(fields) < 3 || fields[0] != "OK" {
		return nil, "", fmt.Errorf("handshake failed: %s", string(buf[:n]))
	}
	id := fields[1]

	session, err := yamux.Client(conn, yamux.DefaultConfig())
	if err != nil {
		return nil, "", err
	}
	return session, id, nil
}

func proxyToLocal(stream net.Conn, port string) {
	defer stream.Close()
	local, err := net.Dial("tcp", "localhost:"+port)
	if err != nil {
		return
	}
	defer local.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(local, stream); done <- struct{}{} }()
	go func() { io.Copy(stream, local); done <- struct{}{} }()
	<-done
}

// ---- Helpers ----

func isUpgradeRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) != "" ||
		strings.ToLower(r.Header.Get("Connection")) == "upgrade" ||
		r.Method == http.MethodConnect
}

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

	if err := r.Write(stream); err != nil {
		return
	}
	if bufrw.Reader.Buffered() > 0 {
		data, _ := bufrw.Reader.Peek(bufrw.Reader.Buffered())
		stream.Write(data)
	}

	errChan := make(chan error, 2)
	go func() { _, err := io.Copy(stream, conn); errChan <- err }()
	go func() { _, err := io.Copy(conn, stream); errChan <- err }()
	<-errChan
}

