package canonicallog

import (
	"context"
	"log/slog"
	"sync"
)

// contextKey is used to store the Accumulator in the context.
// Using an empty struct ensures the key takes up 0 bytes of memory.
type contextKey struct{}

var accumulatorKey = contextKey{}

// Accumulator collects log fields that can be added from anywhere in the request lifecycle.
type Accumulator struct {
	mu     sync.Mutex
	fields []slog.Attr
}

// NewAccumulator creates a new Accumulator and stores it in the context.
// Call this in your interceptor/middleware to bootstrap the per-request accumulator.
func NewAccumulator(ctx context.Context) context.Context {
	// Pre-allocate capacity for 8 fields to prevent early slice reallocations
	acc := &Accumulator{
		fields: make([]slog.Attr, 0, 8),
	}
	return context.WithValue(ctx, accumulatorKey, acc)
}

// GetAccumulator returns the Accumulator from the given context, or nil if not set.
func GetAccumulator(ctx context.Context) *Accumulator {
	acc, _ := ctx.Value(accumulatorKey).(*Accumulator)
	return acc
}

// Add safely appends a field directly to the accumulator instance.
// This encapsulates the locking logic within the struct itself.
func (a *Accumulator) Add(key string, value any) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// slog.Any automatically infers the type of 'value' safely
	a.fields = append(a.fields, slog.Any(key, value))
}

// AddField adds a field to the accumulator stored in the context.
// Safe to call from any goroutine, at any depth of the call stack.
func AddField(ctx context.Context, key string, value any) {
	if acc := GetAccumulator(ctx); acc != nil {
		acc.Add(key, value)
	}
}

// Fields returns a copy of all accumulated fields.
// This prevents race conditions when iterating over the fields later.
func (a *Accumulator) Fields() []slog.Attr {
	a.mu.Lock()
	defer a.mu.Unlock()

	fields := make([]slog.Attr, len(a.fields))
	copy(fields, a.fields)

	return fields
}
