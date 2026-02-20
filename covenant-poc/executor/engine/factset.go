package engine

import (
	"strings"
	"sync"
)

// FactSet is a thread-safe store of named facts gathered during evaluation.
// Fact names are dotted strings like "customer.status" or "payment.amount".
// Facts may be scalars or nested maps (e.g. payment.amount is {"value":500,"currency":"USD"}).
type FactSet struct {
	mu    sync.RWMutex
	facts map[string]any
}

func NewFactSet() *FactSet {
	return &FactSet{facts: make(map[string]any)}
}

// Set stores a fact value by name.
func (f *FactSet) Set(name string, val any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.facts[name] = val
}

// Get returns a fact value by exact name, and whether it was found.
func (f *FactSet) Get(name string) (any, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	v, ok := f.facts[name]
	return v, ok
}

// GetPath resolves a dotted path against the fact set.
// It tries progressively shorter prefixes until it finds a stored fact,
// then navigates into the value using the remaining path segments.
//
// Example: GetPath("payment.amount.value") first checks if "payment.amount.value"
// is a fact; if not, checks "payment.amount" and navigates into its "value" key.
func (f *FactSet) GetPath(path string) (any, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Try exact match first.
	if v, ok := f.facts[path]; ok {
		return v, true
	}

	// Try progressively shorter prefixes.
	parts := strings.Split(path, ".")
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		if v, ok := f.facts[prefix]; ok {
			result, ok := navigatePath(v, parts[i:])
			return result, ok
		}
	}
	return nil, false
}

// Snapshot returns a copy of all facts (for dry-run responses).
func (f *FactSet) Snapshot() map[string]any {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make(map[string]any, len(f.facts))
	for k, v := range f.facts {
		out[k] = v
	}
	return out
}

// navigatePath drills into a nested map/interface value using the given key segments.
func navigatePath(v any, parts []string) (any, bool) {
	for _, part := range parts {
		m, ok := v.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return v, true
}
