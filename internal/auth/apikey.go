package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidAPIKey = errors.New("invalid API key")
	ErrExpiredAPIKey = errors.New("API key has expired")
)

type APIKey struct {
	Key string
	UserID string
	CreatedAt time.Time
	ExpiresAt *time.Time
	Active bool
}

type APIKeyStore struct {
	mu sync.RWMutex
	keys map[string]*APIKey
}

func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{
		keys: make(map[string]*APIKey),
	}
}

// GenerateAPIKey creates a new random API key
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "gw_" + hex.EncodeToString(bytes), nil
}

// CreateAPIKey generates and stores a new API key for a user
func (s *APIKeyStore) CreateAPIKey(userID string, expiresAt *time.Time) (*APIKey, error) {
	key, err := GenerateAPIKey()
	if err != nil {
		return nil, err
	}

	apiKey := &APIKey{
		Key: key,
		UserID: userID,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		Active: true,
	}

	s.mu.Lock()
	s.keys[key] = apiKey
	s.mu.Unlock()

	return apiKey, nil
}

// ValidateAPIKey checks if an API key is valid and active
func (s *APIKeyStore) ValidateAPIKey(key string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apiKey, exists := s.keys[key]
	if !exists {
		return nil, ErrInvalidAPIKey
	}

	if !apiKey.Active {
		return nil, ErrInvalidAPIKey
	}

	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, ErrExpiredAPIKey
	}

	return apiKey, nil
}

// RevokeAPIKey deactivates an API key
func (s *APIKeyStore) RevokeAPIKey(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	apiKey, exists := s.keys[key]
	if !exists {
		return ErrInvalidAPIKey
	}

	apiKey.Active = false
	return nil
}

// LoadSampleKeys loads some sample API keys for testing
func (s *APIKeyStore) LoadSampleKeys() {
	sampleKeys := []*APIKey{
		{
			Key: "gw_test_key_1",
			UserID: "user_1",
			CreatedAt: time.Now(),
			ExpiresAt: nil,
			Active: true,
		},
		{
			Key: "gw_test_key_2",
			UserID: "user_2",
			CreatedAt: time.Now(),
			ExpiresAt: nil,
			Active: true,
		},
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, key := range sampleKeys {
		s.keys[key.Key] = key
	}
}