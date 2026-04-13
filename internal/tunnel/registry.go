package tunnel

import (
	"sync"

	"github.com/hashicorp/yamux"
)

// Registry mantiene el mapa de sesiones yamux activas, indexadas por ID de túnel.
// Es seguro para uso concurrente.
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*yamux.Session
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]*yamux.Session)}
}

func (r *Registry) Add(id string, s *yamux.Session) {
	r.mu.Lock()
	r.entries[id] = s
	r.mu.Unlock()
}

func (r *Registry) Get(id string) (*yamux.Session, bool) {
	r.mu.RLock()
	s, ok := r.entries[id]
	r.mu.RUnlock()
	return s, ok
}

func (r *Registry) Remove(id string) {
	r.mu.Lock()
	delete(r.entries, id)
	r.mu.Unlock()
}
