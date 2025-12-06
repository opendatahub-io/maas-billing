package api_keys

import (
	"context"
	"strings"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of the Store interface.
// FOR UNIT TESTS ONLY - data is lost when the process exits.
type MemoryStore struct {
	mu     sync.RWMutex
	tokens map[string]map[string]ApiKeyMetadata 
}

var _ Store = (*MemoryStore)(nil)

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tokens: make(map[string]map[string]ApiKeyMetadata),
	}
}

func (s *MemoryStore) Close() error {
	return nil
}

func (s *MemoryStore) userKey(namespace, username string) string {
	return namespace + ":" + username
}

func (s *MemoryStore) AddTokenMetadata(_ context.Context, namespace, username string, apiKey *APIKey) error {
	jti := strings.TrimSpace(apiKey.JTI)
	if jti == "" {
		return ErrTokenNotFound
	}

	name := strings.TrimSpace(apiKey.Name)
	if name == "" {
		return ErrTokenNotFound
	}

	var creationDate string
	if apiKey.IssuedAt > 0 {
		creationDate = time.Unix(apiKey.IssuedAt, 0).Format(time.RFC3339)
	} else {
		creationDate = time.Now().Format(time.RFC3339)
	}
	expirationDate := time.Unix(apiKey.ExpiresAt, 0).Format(time.RFC3339)

	metadata := ApiKeyMetadata{
		ID:             jti,
		Name:           name,
		Description:    strings.TrimSpace(apiKey.Description),
		CreationDate:   creationDate,
		ExpirationDate: expirationDate,
		Status:         TokenStatusActive,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.userKey(namespace, username)
	if s.tokens[key] == nil {
		s.tokens[key] = make(map[string]ApiKeyMetadata)
	}
	s.tokens[key][jti] = metadata

	return nil
}

func (s *MemoryStore) MarkTokensAsExpiredForUser(_ context.Context, namespace, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := s.userKey(namespace, username)
	userTokens, exists := s.tokens[key]
	if !exists {
		return nil
	}

	now := time.Now()
	nowStr := now.Format(time.RFC3339)

	for id, t := range userTokens {
		expiration, err := time.Parse(time.RFC3339, t.ExpirationDate)
		if err != nil || now.Before(expiration) {
			t.ExpirationDate = nowStr
			t.Status = TokenStatusExpired
			userTokens[id] = t
		}
	}

	return nil
}

func (s *MemoryStore) GetTokensForUser(_ context.Context, namespace, username string) ([]ApiKeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.userKey(namespace, username)
	userTokens, exists := s.tokens[key]
	if !exists {
		return []ApiKeyMetadata{}, nil
	}

	now := time.Now()
	result := make([]ApiKeyMetadata, 0, len(userTokens))

	for _, t := range userTokens {
		expiration, err := time.Parse(time.RFC3339, t.ExpirationDate)
		if err != nil {
			t.Status = TokenStatusExpired
		} else if now.After(expiration) {
			t.Status = TokenStatusExpired
		} else {
			t.Status = TokenStatusActive
		}
		result = append(result, t)
	}

	return result, nil
}

func (s *MemoryStore) GetToken(_ context.Context, namespace, username, jti string) (*ApiKeyMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.userKey(namespace, username)
	userTokens, exists := s.tokens[key]
	if !exists {
		return nil, ErrTokenNotFound
	}

	t, exists := userTokens[jti]
	if !exists {
		return nil, ErrTokenNotFound
	}

	expiration, err := time.Parse(time.RFC3339, t.ExpirationDate)
	if err != nil {
		t.Status = TokenStatusExpired
	} else if time.Now().After(expiration) {
		t.Status = TokenStatusExpired
	} else {
		t.Status = TokenStatusActive
	}

	return &t, nil
}
