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

// AddIfAbsent registra la entrada sólo si el ID no existe ya. La comprobación
// y la inserción se hacen bajo el mismo lock, evitando la carrera entre un
// Has() previo y el Add() (dos clientes con el mismo subdominio a la vez).
func (r *Registry) AddIfAbsent(id string, entry *TunnelEntry) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[id]; ok {
		return false
	}
	r.entries[id] = entry
	return true
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

// CloseAll cierra todas las sesiones activas. Los handleTunnel que las
// gestionan se desbloquean de su <-session.CloseChan() y hacen la limpieza
// normal (Remove, liberar puertos, etc.). Se usa en el apagado del servidor.
func (r *Registry) CloseAll() {
	r.mu.RLock()
	sessions := make([]*yamux.Session, 0, len(r.entries))
	for _, e := range r.entries {
		if e.Session != nil {
			sessions = append(sessions, e.Session)
		}
	}
	r.mu.RUnlock()
	for _, s := range sessions {
		s.Close()
	}
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
