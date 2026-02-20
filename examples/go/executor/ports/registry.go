package ports

import (
	"context"
	"fmt"
	"sync"
)

// Client is the interface every port adapter must satisfy.
type Client interface {
	// Get retrieves a named fact given the current input (for key extraction).
	Get(ctx context.Context, fact string, input map[string]any) (any, error)
	// Execute performs the operation and returns its output.
	Execute(ctx context.Context, operation string, input map[string]any) (map[string]any, error)
}

// Registry holds named port adapters and implements engine.PortRegistry.
type Registry struct {
	mu      sync.RWMutex
	clients map[string]Client
}

func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]Client)}
}

func (r *Registry) Register(name string, c Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[name] = c
}

func (r *Registry) Get(ctx context.Context, port, fact string, input map[string]any) (any, error) {
	r.mu.RLock()
	c, ok := r.clients[port]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("port %q not registered", port)
	}
	return c.Get(ctx, fact, input)
}

func (r *Registry) Execute(ctx context.Context, port, operation string, input map[string]any) (map[string]any, error) {
	r.mu.RLock()
	c, ok := r.clients[port]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("port %q not registered", port)
	}
	return c.Execute(ctx, operation, input)
}
