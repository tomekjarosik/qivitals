package auth

import (
	"context"
)

// Entity is the interface every authenticated principal must implement.
type Entity interface {
	TokenType() string
	SubjectID() string
	Namespaces() []string
	HasAccessToNamespace(ns string) bool
}

// User represents an authenticated user with Ed25519 keys.
type User struct {
	// ID is the username used in JWT.
	ID string
	// AllowedNamespaces — if nil or empty, user is admin (all namespaces).
	AllowedNamespaces []string
	Type              string
}

func (u *User) TokenType() string    { return u.Type }
func (u *User) SubjectID() string    { return u.ID }
func (u *User) Namespaces() []string { return u.AllowedNamespaces }

// HasAccessToNamespace checks if the user can operate within the given namespace.
// Empty/nil AllowedNamespaces means admin → access all.
func (u *User) HasAccessToNamespace(ns string) bool {
	if ns == "" {
		return true
	}
	if len(u.AllowedNamespaces) == 0 {
		return true // admin
	}
	for _, allowed := range u.AllowedNamespaces {
		if allowed == ns {
			return true
		}
	}
	return false
}

// contextKey is the unexported key type for storing Entity in context.
type contextKey struct{}

// EntityFromContext retrieves the Entity stored in ctx.
func EntityFromContext(ctx context.Context) Entity {
	entity, _ := ctx.Value(contextKey{}).(Entity)
	return entity
}

// NewContextWithEntity attaches an Entity to a context.
func NewContextWithEntity(ctx context.Context, entity Entity) context.Context {
	return context.WithValue(ctx, contextKey{}, entity)
}
