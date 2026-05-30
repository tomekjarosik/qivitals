package auth

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type MagicLinkClaims struct {
	jwt.RegisteredClaims
	LinkID string `json:"link_id"`
	Email  string `json:"email"`
}

type InMemoryMagicLinkStore struct {
	mu    sync.RWMutex
	links map[string]*magicLinkEntry // linkID -> entry
}

type magicLinkEntry struct {
	email     string
	expiresAt time.Time
	used      bool
}

func NewMagicLinkStore() *InMemoryMagicLinkStore {
	return &InMemoryMagicLinkStore{
		links: make(map[string]*magicLinkEntry),
	}
}

func (s *InMemoryMagicLinkStore) Record(linkID, email string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links[linkID] = &magicLinkEntry{
		email:     email,
		expiresAt: expiresAt,
		used:      false,
	}
	return nil
}

func (s *InMemoryMagicLinkStore) Validate(linkID, email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.links[linkID]
	if !exists || entry.used || time.Now().After(entry.expiresAt) {
		return fmt.Errorf("invalid or expired magic link")
	}

	entry.used = true
	delete(s.links, linkID)
	return nil
}
