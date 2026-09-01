package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cdrusu/smuf/internal/tunnel"
	"github.com/hashicorp/yamux"
)

// fakeClient simula el cliente smuf: levanta el extremo yamux que acepta
// streams, lee la petición HTTP reenviada y responde con respFn.
func fakeClient(t *testing.T, registry *tunnel.Registry, id string, respFn func(*http.Request) *http.Response) {
	t.Helper()

	srvConn, cliConn := net.Pipe()

	// El proxy llama entry.Session.Open(); ese extremo es el "servidor" yamux.
	serverSession, err := yamux.Server(srvConn, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux.Server: %v", err)
	}
	clientSession, err := yamux.Client(cliConn, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}

	registry.Add(id, &tunnel.TunnelEntry{
		Session:   serverSession,
		Type:      tunnel.TunnelHTTP,
		Port:      "3000",
		PublicURL: "http://" + id + ".example.com",
		ClientIP:  "127.0.0.1",
		CreatedAt: time.Now(),
	})

	go func() {
		for {
			stream, err := clientSession.Accept()
			if err != nil {
				return
			}
			go func(s net.Conn) {
				defer s.Close()
				req, err := http.ReadRequest(bufio.NewReader(s))
				if err != nil {
					return
				}
				resp := respFn(req)
				resp.Write(s)
			}(stream)
		}
	}()

	t.Cleanup(func() {
		clientSession.Close()
		serverSession.Close()
		registry.Remove(id)
	})
}

func okResponse(body string) func(*http.Request) *http.Response {
	return func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode:    http.StatusOK,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header{"Content-Type": []string{"text/plain"}},
			Body:          io.NopCloser(strings.NewReader(body)),
			ContentLength: int64(len(body)),
		}
	}
}

func TestProxyForwardsRequestAndResponse(t *testing.T) {
	registry := tunnel.NewRegistry()
	fakeClient(t, registry, "abc123", okResponse("hello from local"))

	proxy := newHTTPProxy("example.com", registry)

	req := httptest.NewRequest(http.MethodGet, "http://abc123.example.com/path", nil)
	req.Host = "abc123.example.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "hello from local" {
		t.Fatalf("body = %q, want %q", got, "hello from local")
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
}

func TestProxyPropagatesStatusAndHeaders(t *testing.T) {
	registry := tunnel.NewRegistry()
	fakeClient(t, registry, "sub", func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode:    http.StatusTeapot,
			Proto:         "HTTP/1.1",
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header{"X-Custom": []string{"yes"}},
			Body:          io.NopCloser(strings.NewReader("")),
			ContentLength: 0,
		}
	})

	proxy := newHTTPProxy("example.com", registry)

	req := httptest.NewRequest(http.MethodPost, "http://sub.example.com/", nil)
	req.Host = "sub.example.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want 418", rec.Code)
	}
	if rec.Header().Get("X-Custom") != "yes" {
		t.Fatalf("X-Custom header not propagated")
	}
}

func TestProxyUnknownTunnelReturns404(t *testing.T) {
	proxy := newHTTPProxy("example.com", tunnel.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "http://nope.example.com/", nil)
	req.Host = "nope.example.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestProxyMissingSubdomainReturns400(t *testing.T) {
	proxy := newHTTPProxy("example.com", tunnel.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "http://other-domain.com/", nil)
	req.Host = "other-domain.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestProxyRootHostServesDashboard(t *testing.T) {
	proxy := newHTTPProxy("example.com", tunnel.NewRegistry())

	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rec.Body.String(), "smuf") {
		t.Fatalf("dashboard body does not contain 'smuf'")
	}
}

func TestProxyTunnelsJSONEndpoint(t *testing.T) {
	registry := tunnel.NewRegistry()
	fakeClient(t, registry, "json1", okResponse("x"))

	proxy := newHTTPProxy("example.com", registry)

	req := httptest.NewRequest(http.MethodGet, "http://example.com/_smuf/tunnels", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), "json1") {
		t.Fatalf("JSON does not contain tunnel id, got: %s", rec.Body.String())
	}
}

// fakeRawClient es como fakeClient pero deja que el test maneje el stream a
// pelo: sirve para respuestas en streaming y para upgrades (101).
func fakeRawClient(t *testing.T, registry *tunnel.Registry, id string, handle func(net.Conn)) {
	t.Helper()

	srvConn, cliConn := net.Pipe()

	serverSession, err := yamux.Server(srvConn, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux.Server: %v", err)
	}
	clientSession, err := yamux.Client(cliConn, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}

	registry.Add(id, &tunnel.TunnelEntry{
		Session:   serverSession,
		Type:      tunnel.TunnelHTTP,
		Port:      "3000",
		PublicURL: "http://" + id + ".example.com",
		ClientIP:  "127.0.0.1",
		CreatedAt: time.Now(),
	})

	go func() {
		for {
			stream, err := clientSession.Accept()
			if err != nil {
				return
			}
			go handle(stream)
		}
	}()

	t.Cleanup(func() {
		clientSession.Close()
		serverSession.Close()
		registry.Remove(id)
	})
}

// TestProxyStreamsResponseWithoutBuffering cubre el fallo por el que una
// respuesta SSE no llegaba al cliente hasta acumular 4 KB o terminar: el
// proxy copiaba el cuerpo sin hacer Flush.
func TestProxyStreamsResponseWithoutBuffering(t *testing.T) {
	registry := tunnel.NewRegistry()
	secondChunk := make(chan struct{})

	fakeRawClient(t, registry, "sse", func(s net.Conn) {
		defer s.Close()
		if _, err := http.ReadRequest(bufio.NewReader(s)); err != nil {
			return
		}
		io.WriteString(s, "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\n\r\n")
		io.WriteString(s, "data: uno\n\n")
		// El segundo trozo no sale hasta que el test confirme que ya recibió
		// el primero; si el proxy bufferizara, el test se quedaría aquí.
		<-secondChunk
		io.WriteString(s, "data: dos\n\n")
	})

	srv := httptest.NewServer(newHTTPProxy("example.com", registry))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "sse.example.com"

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	type readResult struct {
		line string
		err  error
	}
	lines := make(chan readResult, 2)
	reader := bufio.NewReader(resp.Body)
	go func() {
		for i := 0; i < 2; i++ {
			line, err := reader.ReadString('\n')
			lines <- readResult{line: line, err: err}
			if err != nil {
				return
			}
		}
	}()

	select {
	case got := <-lines:
		if got.err != nil {
			t.Fatalf("primer chunk: %v", got.err)
		}
		if strings.TrimSpace(got.line) != "data: uno" {
			t.Fatalf("primer chunk = %q, want %q", got.line, "data: uno")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("el primer chunk no llegó: la respuesta se está bufferizando")
	}

	close(secondChunk)

	select {
	case got := <-lines:
		if got.err != nil {
			t.Fatalf("segundo chunk: %v", got.err)
		}
		if strings.TrimSpace(got.line) != "" && strings.TrimSpace(got.line) != "data: dos" {
			t.Fatalf("segundo chunk inesperado: %q", got.line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("el segundo chunk no llegó")
	}
}

// TestProxyAddsForwardedHeaders comprueba que la app local recibe las
// cabeceras X-Forwarded-*, necesarias para que genere URLs correctas cuando
// el servidor público termina TLS.
func TestProxyAddsForwardedHeaders(t *testing.T) {
	registry := tunnel.NewRegistry()
	got := make(chan http.Header, 1)

	fakeRawClient(t, registry, "fwd", func(s net.Conn) {
		defer s.Close()
		req, err := http.ReadRequest(bufio.NewReader(s))
		if err != nil {
			return
		}
		req.Header.Set("X-Seen-Host", req.Host)
		got <- req.Header
		io.WriteString(s, "HTTP/1.1 204 No Content\r\nContent-Length: 0\r\n\r\n")
	})

	proxy := newHTTPProxy("example.com", registry)
	req := httptest.NewRequest(http.MethodGet, "http://fwd.example.com/x", nil)
	req.Host = "fwd.example.com"
	req.RemoteAddr = "203.0.113.7:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // debe ser sustituida, no heredada a ciegas
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	select {
	case h := <-got:
		if h.Get("X-Forwarded-Proto") != "http" {
			t.Fatalf("X-Forwarded-Proto = %q, want http", h.Get("X-Forwarded-Proto"))
		}
		if h.Get("X-Forwarded-Host") != "fwd.example.com" {
			t.Fatalf("X-Forwarded-Host = %q", h.Get("X-Forwarded-Host"))
		}
		if !strings.HasSuffix(h.Get("X-Forwarded-For"), "203.0.113.7") {
			t.Fatalf("X-Forwarded-For = %q, want que acabe en la IP real", h.Get("X-Forwarded-For"))
		}
		if h.Get("X-Seen-Host") != "fwd.example.com" {
			t.Fatalf("Host visto por la app = %q, want fwd.example.com", h.Get("X-Seen-Host"))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("la petición no llegó al cliente falso")
	}
}

// TestProxyWebSocketUpgrade cubre el camino de upgrade (101), que ahora
// gestiona ReverseProxy en lugar del hijack manual.
func TestProxyWebSocketUpgrade(t *testing.T) {
	registry := tunnel.NewRegistry()

	fakeRawClient(t, registry, "ws", func(s net.Conn) {
		defer s.Close()
		if _, err := http.ReadRequest(bufio.NewReader(s)); err != nil {
			return
		}
		io.WriteString(s, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
		io.Copy(s, s) // eco de bytes crudos
	})

	srv := httptest.NewServer(newHTTPProxy("example.com", registry))
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	io.WriteString(conn, "GET /ws HTTP/1.1\r\nHost: ws.example.com\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("leer respuesta: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}

	io.WriteString(conn, "ping")
	buf := make([]byte, 4)
	if _, err := io.ReadFull(br, buf); err != nil {
		t.Fatalf("leer eco: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("eco = %q, want ping", buf)
	}
}
