package tunnel

import (
	"sync"
	"time"

	"github.com/hashicorp/yamux"
)

// TunnelEntry holds the yamux session and metadata for an active tunnel.
type TunnelEntry struct {
	Session   *yamux.Session
	Port      string
	PublicURL string
	ClientIP  string
	CreatedAt time.Time
}

// TunnelInfo is a JSON-safe snapshot of tunnel metadata.
type TunnelInfo struct {
	ID        string    `json:"id"`
	Port      string    `json:"port"`
	PublicURL string    `json:"public_url"`
	ClientIP  string    `json:"client_ip"`
	CreatedAt time.Time `json:"created_at"`
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

// List returns a snapshot of all active tunnel metadata.
func (r *Registry) List() []TunnelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TunnelInfo, 0, len(r.entries))
	for id, e := range r.entries {
		out = append(out, TunnelInfo{
			ID:        id,
			Port:      e.Port,
			PublicURL: e.PublicURL,
			ClientIP:  e.ClientIP,
			CreatedAt: e.CreatedAt,
		})
	}
	return out
}
