package tunnel

import (
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

// Stub para satisfacer la compilación sin una sesión real.
var _ = yamux.Session{}
