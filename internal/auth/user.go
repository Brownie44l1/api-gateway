package auth

import (
	"errors"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
)

// InMemoryUserStore is a simple in-memory user store for testing
type InMemoryUserStore struct {
	mu    sync.RWMutex
	users map[string]*User
	// username -> userID mapping
	usernameToID map[string]string
	// hashed passwords
	passwords map[string]string
}

// NewInMemoryUserStore creates a new in-memory user store
func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{
		users:        make(map[string]*User),
		usernameToID: make(map[string]string),
		passwords:    make(map[string]string),
	}
}

// CreateUser adds a user to the store
func (s *InMemoryUserStore) CreateUser(username, password, email string, roles, permissions []string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate user ID
	userID := "user_" + username

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:          userID,
		Username:    username,
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
	}

	s.users[userID] = user
	s.usernameToID[username] = userID
	s.passwords[userID] = string(hashedPassword)

	return user, nil
}

// ValidateCredentials checks if username and password are valid
func (s *InMemoryUserStore) ValidateCredentials(username, password string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userID, ok := s.usernameToID[username]
	if !ok {
		return nil, ErrInvalidCredentials
	}

	hashedPassword, ok := s.passwords[userID]
	if !ok {
		return nil, ErrInvalidCredentials
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	user, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *InMemoryUserStore) GetUser(userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return nil, ErrUserNotFound
	}

	return user, nil
}

// LoadSampleUsers creates some test users
func (s *InMemoryUserStore) LoadSampleUsers() error {
	users := []struct {
		username    string
		password    string
		email       string
		roles       []string
		permissions []string
	}{
		{
			username:    "admin",
			password:    "admin123",
			email:       "admin@example.com",
			roles:       []string{"admin", "user"},
			permissions: []string{"read", "write", "delete"},
		},
		{
			username:    "user1",
			password:    "password123",
			email:       "user1@example.com",
			roles:       []string{"user"},
			permissions: []string{"read", "write"},
		},
		{
			username:    "readonly",
			password:    "readonly123",
			email:       "readonly@example.com",
			roles:       []string{"viewer"},
			permissions: []string{"read"},
		},
	}

	for _, u := range users {
		if _, err := s.CreateUser(u.username, u.password, u.email, u.roles, u.permissions); err != nil {
			return err
		}
	}

	return nil
}
