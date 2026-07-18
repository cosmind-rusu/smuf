package tunnel

import (
	"net"
	"testing"
	"time"

	"github.com/hashicorp/yamux"
)

func TestRegistryAddGetRemove(t *testing.T) {
	r := NewRegistry()
	e := &TunnelEntry{
		Port:      "3000",
		PublicURL: "http://test.localhost:8080",
		ClientIP:  "127.0.0.1",
		CreatedAt: time.Now(),
	}

	r.Add("abc123", e)

	got, ok := r.Get("abc123")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if got.Port != "3000" {
		t.Fatalf("expected port 3000, got %s", got.Port)
	}

	r.Remove("abc123")
	_, ok = r.Get("abc123")
	if ok {
		t.Fatal("expected entry to be removed")
	}
}

func TestRegistryHas(t *testing.T) {
	r := NewRegistry()
	r.Add("foo", &TunnelEntry{Port: "8080", CreatedAt: time.Now()})

	if !r.Has("foo") {
		t.Error("expected Has(foo) = true")
	}
	if r.Has("bar") {
		t.Error("expected Has(bar) = false")
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()
	r.Add("a", &TunnelEntry{Port: "1", PublicURL: "u1", ClientIP: "ip1", CreatedAt: time.Now()})
	r.Add("b", &TunnelEntry{Port: "2", PublicURL: "u2", ClientIP: "ip2", CreatedAt: time.Now()})

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
}

func TestRegistryConcurrent(t *testing.T) {
	r := NewRegistry()
	// Simular uso concurrente básico sin panic
	go r.Add("x", &TunnelEntry{Port: "1", CreatedAt: time.Now()})
	go r.Add("y", &TunnelEntry{Port: "2", CreatedAt: time.Now()})
	go func() { r.Get("x") }()
	go func() { r.List() }()
	time.Sleep(50 * time.Millisecond)
}

func TestRegistryAddIfAbsent(t *testing.T) {
	r := NewRegistry()
	if !r.AddIfAbsent("dup", &TunnelEntry{Port: "1", CreatedAt: time.Now()}) {
		t.Fatal("first AddIfAbsent should succeed")
	}
	if r.AddIfAbsent("dup", &TunnelEntry{Port: "2", CreatedAt: time.Now()}) {
		t.Fatal("second AddIfAbsent with the same ID should fail")
	}
	got, ok := r.Get("dup")
	if !ok {
		t.Fatal("expected entry to exist")
	}
	if got.Port != "1" {
		t.Fatalf("existing entry was overwritten: port = %s, want 1", got.Port)
	}
}

func TestRegistryCloseAll(t *testing.T) {
	r := NewRegistry()

	// Sesión yamux real sobre net.Pipe para comprobar que CloseAll la cierra.
	srvConn, cliConn := net.Pipe()
	serverSession, err := yamux.Server(srvConn, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux.Server: %v", err)
	}
	clientSession, err := yamux.Client(cliConn, yamux.DefaultConfig())
	if err != nil {
		t.Fatalf("yamux.Client: %v", err)
	}
	defer clientSession.Close()

	r.Add("t1", &TunnelEntry{Session: serverSession, CreatedAt: time.Now()})
	r.Add("t2", &TunnelEntry{CreatedAt: time.Now()}) // sin sesión: no debe romper

	r.CloseAll()

	select {
	case <-serverSession.CloseChan():
	case <-time.After(2 * time.Second):
		t.Fatal("CloseAll did not close the session")
	}
}

// Stub para satisfacer la compilación sin una sesión real.
var _ = yamux.Session{}
