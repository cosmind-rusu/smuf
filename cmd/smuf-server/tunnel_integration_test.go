package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/hashicorp/yamux"
)

// startTestControl levanta handleTunnel real sobre un listener TCP local,
// igual que hace startControl pero sin rate limiters.
func startTestControl(t *testing.T, registry *tunnel.Registry) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	cfg := serverConfig{
		domain:           "example.com",
		httpPort:         "8080",
		handshakeTimeout: 5 * time.Second,
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleTunnel(conn, cfg, registry, nil)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

// handshakeClient hace el handshake exactamente como lo hace el cliente smuf
// y devuelve la sesión yamux ya establecida.
func handshakeClient(t *testing.T, addr, handshake string) *yamux.Session {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	fmt.Fprintf(conn, "%s\n", handshake)
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read handshake response: %v", err)
	}
	if !strings.HasPrefix(line, "OK ") {
		t.Fatalf("handshake rejected: %s", strings.TrimSpace(line))
	}
	session, err := yamux.Client(tunnel.NewBufConn(conn, reader), yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux client: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func waitForEntry(t *testing.T, registry *tunnel.Registry) *tunnel.TunnelEntry {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if list := registry.List(); len(list) == 1 {
			if e, ok := registry.Get(list[0].ID); ok {
				return e
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not register the tunnel")
	return nil
}

// TestTunnelSurvivesMoreThanHandshakeLimit reproduce el bug del LimitReader:
// el handshake lee la conexión a través de un io.LimitReader de 1KB, y ese
// límite quedaba en la cadena de lectura de yamux, así que la sesión moría
// con EOF tras ~1KB de tráfico cliente→servidor. Aquí se envían 8KB.
func TestTunnelSurvivesMoreThanHandshakeLimit(t *testing.T) {
	registry := tunnel.NewRegistry()
	addr := startTestControl(t, registry)

	session := handshakeClient(t, addr, "PORT 3000")
	entry := waitForEntry(t, registry)

	// El servidor acepta un stream y lee el payload completo.
	const payloadSize = 8 * 1024
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	go func() {
		stream, err := entry.Session.Accept()
		if err != nil {
			done <- result{err: fmt.Errorf("server accept: %w", err)}
			return
		}
		defer stream.Close()
		buf := make([]byte, payloadSize)
		n, err := io.ReadFull(stream, buf)
		if err != nil {
			done <- result{n: n, err: fmt.Errorf("server read: %w", err)}
			return
		}
		done <- result{n: n}
	}()

	// El cliente abre un stream y escribe 8KB (≫ 1KB del antiguo límite).
	stream, err := session.Open()
	if err != nil {
		t.Fatalf("client open stream: %v", err)
	}
	payload := bytes.Repeat([]byte("a"), payloadSize)
	if _, err := stream.Write(payload); err != nil {
		t.Fatalf("client write: %v", err)
	}

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("tunnel died after %d bytes (LimitReader bug?): %v", r.n, r.err)
		}
		if r.n != payloadSize {
			t.Fatalf("server read %d bytes, want %d", r.n, payloadSize)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: server did not receive the payload (tunnel dead?)")
	}
}

// TestDuplicateSubdomainRejected verifica que un segundo cliente con el mismo
// subdominio es rechazado con un error claro.
func TestDuplicateSubdomainRejected(t *testing.T) {
	registry := tunnel.NewRegistry()
	addr := startTestControl(t, registry)

	handshakeClient(t, addr, "PORT 3000 SUB myapp")
	waitForEntry(t, registry)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "PORT 3001 SUB myapp\n")
	line, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !strings.HasPrefix(line, "ERR subdomain in use") {
		t.Fatalf("expected 'ERR subdomain in use', got %q", strings.TrimSpace(line))
	}
}
