package main

import (
	"testing"
	"time"

	"github.com/cdrusu/smuf/internal/tunnel"
)

func TestConnRateAttemptLimiterBlocksBursts(t *testing.T) {
	l := newConnRateAttemptLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !l.allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed within limit", i)
		}
	}
	if l.allow("1.2.3.4") {
		t.Fatal("4th attempt within the window should be blocked")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("a different IP should have its own budget")
	}
}

func TestConnRateAttemptLimiterExpiresOldAttempts(t *testing.T) {
	l := newConnRateAttemptLimiter(1, 10*time.Millisecond)

	if !l.allow("1.2.3.4") {
		t.Fatal("first attempt should be allowed")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("second attempt inside the window should be blocked")
	}
	time.Sleep(20 * time.Millisecond)
	if !l.allow("1.2.3.4") {
		t.Fatal("attempt after the window elapsed should be allowed again")
	}
}

func TestConnRateAttemptLimiterSweepRemovesStaleIPs(t *testing.T) {
	l := newConnRateAttemptLimiter(1, 10*time.Millisecond)
	l.allow("1.2.3.4")
	time.Sleep(20 * time.Millisecond)

	l.sweep()

	l.mu.Lock()
	_, ok := l.attempts["1.2.3.4"]
	l.mu.Unlock()
	if ok {
		t.Fatal("sweep should have removed the stale IP entry")
	}
}

func TestNewUniqueIDAvoidsRegistryCollisions(t *testing.T) {
	registry := tunnel.NewRegistry()

	id, err := newUniqueID(registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected a non-empty id")
	}

	// Ocupa el ID recién generado para forzar que la próxima llamada lo evite.
	registry.Add(id, &tunnel.TunnelEntry{})

	second, err := newUniqueID(registry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second == id {
		t.Fatal("newUniqueID must not return an ID already present in the registry")
	}
}
