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

	proxy := &httpProxy{domain: "example.com", registry: registry}

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

	proxy := &httpProxy{domain: "example.com", registry: registry}

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
	proxy := &httpProxy{domain: "example.com", registry: tunnel.NewRegistry()}

	req := httptest.NewRequest(http.MethodGet, "http://nope.example.com/", nil)
	req.Host = "nope.example.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestProxyMissingSubdomainReturns400(t *testing.T) {
	proxy := &httpProxy{domain: "example.com", registry: tunnel.NewRegistry()}

	req := httptest.NewRequest(http.MethodGet, "http://other-domain.com/", nil)
	req.Host = "other-domain.com"
	rec := httptest.NewRecorder()

	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestProxyRootHostServesDashboard(t *testing.T) {
	proxy := &httpProxy{domain: "example.com", registry: tunnel.NewRegistry()}

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

	proxy := &httpProxy{domain: "example.com", registry: registry}

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
