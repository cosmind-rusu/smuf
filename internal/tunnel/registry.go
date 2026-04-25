package tunnel

import (
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// TunnelType indica si el túnel es HTTP o TCP puro.
type TunnelType string

const (
	TunnelHTTP TunnelType = "http"
	TunnelTCP  TunnelType = "tcp"
)

// TunnelEntry holds the yamux session and metadata for an active tunnel.
type TunnelEntry struct {
	Session      *yamux.Session
	Type         TunnelType
	Port         string
	PublicURL    string
	PublicTCPPort string // solo para TCP puro
	ClientIP     string
	CreatedAt    time.Time
}

// TunnelInfo is a JSON-safe snapshot of tunnel metadata.
type TunnelInfo struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Port          string    `json:"port"`
	PublicURL     string    `json:"public_url"`
	PublicTCPPort string    `json:"public_tcp_port,omitempty"`
	ClientIP      string    `json:"client_ip"`
	CreatedAt     time.Time `json:"created_at"`
}

// Registry mantiene el mapa de sesiones yamux activas, indexadas por ID de túnel.
// Es seguro para uso concurrente.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*TunnelEntry
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*TunnelEntry)}
}

func (r *Registry) Add(id string, entry *TunnelEntry) {
	r.mu.Lock()
	r.entries[id] = entry
	r.mu.Unlock()
}

func (r *Registry) Get(id string) (*TunnelEntry, bool) {
	r.mu.RLock()
	e, ok := r.entries[id]
	r.mu.RUnlock()
	return e, ok
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	delete(r.entries, id)
	r.mu.Unlock()
}

// Has reports whether an ID is already registered.
func (r *Registry) Has(id string) bool {
	r.mu.RLock()
	_, ok := r.entries[id]
	r.mu.RUnlock()
	return ok
}

// List returns a snapshot of all active tunnel metadata.
func (r *Registry) List() []TunnelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TunnelInfo, 0, len(r.entries))
	for id, e := range r.entries {
		out = append(out, TunnelInfo{
			ID:            id,
			Type:          string(e.Type),
			Port:          e.Port,
			PublicURL:     e.PublicURL,
			PublicTCPPort: e.PublicTCPPort,
			ClientIP:      e.ClientIP,
			CreatedAt:     e.CreatedAt,
		})
	}
	return out
}
