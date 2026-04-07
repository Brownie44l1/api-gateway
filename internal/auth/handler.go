package auth

import (
	"encoding/json"
	"net/http"

	jwtpkg "github.com/Brownie44l1/api-gateway/internal/jwt"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	tokenManager *jwtpkg.TokenManager
	userStore    UserStore // Interface for user lookup
}

// UserStore interface for user authentication
type UserStore interface {
	// ValidateCredentials checks username/password
	ValidateCredentials(username, password string) (*User, error)
	// GetUser retrieves user by ID
	GetUser(userID string) (*User, error)
}

// User represents a user in the system
type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

// LoginRequest represents login credentials
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse contains access and refresh tokens
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds
}

// RefreshRequest contains refresh token
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// RefreshResponse contains new access token
type RefreshResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(tokenManager *jwtpkg.TokenManager, userStore UserStore) *AuthHandler {
	return &AuthHandler{
		tokenManager: tokenManager,
		userStore:    userStore,
	}
}

// Login handles user login and token generation
func (ah *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate credentials
	user, err := ah.userStore.ValidateCredentials(req.Username, req.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate tokens
	accessToken, err := ah.tokenManager.GenerateAccessToken(
		user.ID,
		user.Email,
		user.Roles,
		user.Permissions,
	)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	refreshToken, err := ah.tokenManager.GenerateRefreshToken(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate refresh token", http.StatusInternalServerError)
		return
	}

	// Return tokens
	response := LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(ah.tokenManager.GetAccessTokenTTL().Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Refresh handles token refresh
func (ah *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// FIXED: Use ValidateRefreshToken instead of ValidateToken
	// Refresh tokens have a simpler structure (just user ID in subject)
	userID, err := ah.tokenManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	// Get user to fetch current roles/permissions
	user, err := ah.userStore.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusUnauthorized)
		return
	}

	// Generate new access token with current roles/permissions
	accessToken, err := ah.tokenManager.GenerateAccessToken(
		user.ID,
		user.Email,
		user.Roles,
		user.Permissions,
	)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	response := RefreshResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(ah.tokenManager.GetAccessTokenTTL().Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Me returns current user info
func (ah *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := jwtpkg.GetClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := ah.userStore.GetUser(claims.UserID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}