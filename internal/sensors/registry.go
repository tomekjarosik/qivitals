package sensors

import (
	"fmt"
	"sort"
	"sync"
)

// Constructor is a function that creates a new SensorReader instance
// with the given arguments. Each sensor type provides its own constructor.
type Constructor func(args ...string) (SensorReader, error)

// Registry manages all registered sensor types. It is thread-safe
// and intended to be shared across the application.
type Registry struct {
	mu           sync.RWMutex
	constructors map[string]Constructor
}

var registry = &Registry{
	constructors: make(map[string]Constructor),
}

// Global registry instance. Register your sensor constructors
// in init() functions so they're available at startup.

// NewRegistry creates a new empty registry. Use the global
// DefaultRegistry() for shared access.
func NewRegistry() *Registry {
	return &Registry{
		constructors: make(map[string]Constructor),
	}
}

// Register adds a sensor type to the registry. If the name
// is already registered, it overwrites the previous constructor.
func (r *Registry) Register(name string, constructor Constructor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.constructors[name] = constructor
}

// Create instantiates a new sensor of the given type with the
// provided arguments. Returns an error if the type is unknown.
func (r *Registry) Create(name string, args ...string) (SensorReader, error) {
	r.mu.RLock()
	constructor, ok := r.constructors[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown sensor type %q", name)
	}

	return constructor(args...)
}

// RegisterDefault is a convenience function for the global registry.
func RegisterDefault(name string, constructor Constructor) {
	registry.Register(name, constructor)
}

// AvailableTypes returns a sorted list of all registered sensor type names.
func (r *Registry) AvailableTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.constructors))
	for name := range r.constructors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultRegistry returns the global registry instance.
func DefaultRegistry() *Registry {
	return registry
}
