package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/hashicorp/yamux"
)

// Test manual de túnel TCP puro.
// Ejecutar: go run ./cmd/test-tcp
//
// 1. Levanta un servidor TCP echo en localhost:19998
// 2. Levanta smuf-server con rango TCP 25000-25100
// 3. Cliente smuf conecta y pide túnel TCP para :19998
// 4. Conecta un cliente TCP al puerto público asignado
// 5. Verifica eco

func main() {
	// 1. Servidor echo TCP
	go startTCPEchoServer(":19998")
	fmt.Println("[test] TCP echo server on :19998")

	// 2. smuf-server con TCP habilitado
	registry := tunnel.NewRegistry()
	go startControlServer(":17001", registry)
	fmt.Println("[test] smuf-server control on :17001")

	time.Sleep(300 * time.Millisecond)

	// 3. Cliente smuf
	session, tunnelID, publicPort, err := connectTCPClient(":17001", "19998")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] client failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[test] tunnel %s public tcp port %s\n", tunnelID, publicPort)

	// Aceptar streams y proxear
	go func() {
		for {
			stream, err := session.Accept()
			if err != nil {
				return
			}
			go proxyToLocal(stream, "19998")
		}
	}()

	// 4. Cliente TCP al puerto público
	publicAddr := "127.0.0.1:" + publicPort
	fmt.Printf("[test] dialing public tcp %s\n", publicAddr)
	conn, err := net.Dial("tcp", publicAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] dial public failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	// 5. Enviar y recibir eco
	fmt.Fprintf(conn, "hola\n")
	resp, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "[test] read failed: %v\n", err)
		os.Exit(1)
	}
	if strings.TrimSpace(resp) != "echo: hola" {
		fmt.Fprintf(os.Stderr, "[test] expected 'echo: hola', got %q\n", strings.TrimSpace(resp))
		os.Exit(1)
	}

	fmt.Println("[test] ✅ TCP tunnel test passed!")
}

func startTCPEchoServer(addr string) {
	ln, _ := net.Listen("tcp", addr)
	for {
		conn, _ := ln.Accept()
		go func(c net.Conn) {
			defer c.Close()
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				fmt.Fprintf(c, "echo: %s\n", scanner.Text())
			}
		}(conn)
	}
}

func startControlServer(addr string, registry *tunnel.Registry) {
	ln, _ := net.Listen("tcp", addr)
	for {
		conn, _ := ln.Accept()
		go handleControl(conn, registry)
	}
}

func handleControl(conn net.Conn, registry *tunnel.Registry) {
	defer conn.Close()
	buf := make([]byte, 128)
	n, _ := conn.Read(buf)
	line := strings.TrimSpace(string(buf[:n]))
	parts := strings.Fields(line)
	if len(parts) < 2 || parts[0] != "TCP" {
		fmt.Fprintf(conn, "ERR invalid handshake\n")
		return
	}
	port := parts[1]
	id := "testtcp01"

	// Asignar puerto público (hardcode 25001 para test)
	publicPort := "25001"
	fmt.Fprintf(conn, "OK %s tcp://localhost:%s\n", id, publicPort)

	session, _ := yamux.Server(conn, yamux.DefaultConfig())
	defer session.Close()

	registry.Add(id, &tunnel.TunnelEntry{
		Session:       session,
		Type:          tunnel.TunnelTCP,
		Port:          port,
		PublicURL:     "tcp://localhost:" + publicPort,
		PublicTCPPort: publicPort,
	})
	defer registry.Remove(id)

	// Listener TCP público
	ln, err := net.Listen("tcp", ":"+publicPort)
	if err != nil {
		return
	}
	defer ln.Close()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(clientConn net.Conn) {
				defer clientConn.Close()
				stream, err := session.Open()
				if err != nil {
					return
				}
				defer stream.Close()
				done := make(chan struct{}, 2)
				go func() { io.Copy(stream, clientConn); done <- struct{}{} }()
				go func() { io.Copy(clientConn, stream); done <- struct{}{} }()
				<-done
			}(c)
		}
	}()

	<-session.CloseChan()
}

func connectTCPClient(serverAddr, localPort string) (*yamux.Session, string, string, error) {
	conn, err := net.Dial("tcp", serverAddr)
	if err != nil {
		return nil, "", "", err
	}
	fmt.Fprintf(conn, "TCP %s\n", localPort)
	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, "", "", err
	}
	fields := strings.Fields(string(buf[:n]))
	if len(fields) < 3 || fields[0] != "OK" {
		return nil, "", "", fmt.Errorf("handshake failed: %s", string(buf[:n]))
	}
	id := fields[1]
	publicURL := fields[2]
	publicPort := ""
	if strings.HasPrefix(publicURL, "tcp://") {
		parts := strings.Split(publicURL, ":")
		if len(parts) >= 3 {
			publicPort = parts[2]
		}
	}

	session, err := yamux.Client(conn, yamux.DefaultConfig())
	if err != nil {
		return nil, "", "", err
	}
	return session, id, publicPort, nil
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
