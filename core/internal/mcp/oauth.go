package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// OAuthToken represents a stored OAuth token
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Scopes       []string  `json:"scopes,omitempty"`
}

// OAuthConfig describes an OAuth2 configuration for an MCP server
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

// McpOAuthManager handles OAuth2 flows for MCP servers
type McpOAuthManager struct {
	mu     sync.RWMutex
	tokens map[string]*OAuthToken // serverName -> token
	path   string
}

// NewMcpOAuthManager creates a new OAuth manager
func NewMcpOAuthManager() (*McpOAuthManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	configDir := filepath.Join(homeDir, ".ricochet")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir: %w", err)
	}

	mgr := &McpOAuthManager{
		tokens: make(map[string]*OAuthToken),
		path:   filepath.Join(configDir, "mcp_tokens.json"),
	}

	if err := mgr.Load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load tokens: %w", err)
		}
	}

	return mgr, nil
}

// Load reads tokens from disk
func (m *McpOAuthManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return err
	}

	var tokens map[string]*OAuthToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		return fmt.Errorf("failed to parse mcp_tokens.json: %w", err)
	}

	m.tokens = tokens
	return nil
}

// Save writes tokens to disk
func (m *McpOAuthManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.tokens, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tokens: %w", err)
	}

	return os.WriteFile(m.path, data, 0600) // Restrictive permissions for security
}

// GetToken retrieves a token for a server
func (m *McpOAuthManager) GetToken(serverName string) (*OAuthToken, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, ok := m.tokens[serverName]
	if !ok {
		return nil, false
	}

	// Check if token is expired
	if !token.ExpiresAt.IsZero() && token.ExpiresAt.Before(time.Now()) {
		return nil, false // Token expired
	}

	return token, true
}

// SetToken stores a token for a server
func (m *McpOAuthManager) SetToken(serverName string, token *OAuthToken) error {
	m.mu.Lock()
	m.tokens[serverName] = token
	m.mu.Unlock()

	return m.Save()
}

// RemoveToken deletes a token for a server
func (m *McpOAuthManager) RemoveToken(serverName string) error {
	m.mu.Lock()
	delete(m.tokens, serverName)
	m.mu.Unlock()

	return m.Save()
}

// NeedsAuth checks if a server requires authentication
func (m *McpOAuthManager) NeedsAuth(serverName string) bool {
	_, ok := m.GetToken(serverName)
	return !ok
}

// StartOAuthFlow initiates an OAuth2 authorization flow
// Returns the authorization URL for the user to visit
func (m *McpOAuthManager) StartOAuthFlow(serverName string, config *OAuthConfig) (string, string, error) {
	// Generate state for CSRF protection
	state := fmt.Sprintf("%s-%d", serverName, time.Now().UnixNano())

	// Build authorization URL
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&state=%s",
		config.AuthURL,
		config.ClientID,
		config.RedirectURL,
		state,
	)

	if len(config.Scopes) > 0 {
		scopes := ""
		for i, s := range config.Scopes {
			if i > 0 {
				scopes += " "
			}
			scopes += s
		}
		authURL += "&scope=" + scopes
	}

	return authURL, state, nil
}

// ExchangeCode exchanges an authorization code for tokens
// This is a stub - actual implementation would make HTTP request to token endpoint
func (m *McpOAuthManager) ExchangeCode(serverName string, code string, config *OAuthConfig) (*OAuthToken, error) {
	// TODO: Implement actual OAuth2 token exchange
	// This would involve:
	// 1. POST to config.TokenURL with code, client_id, client_secret
	// 2. Parse response for access_token, refresh_token, expires_in
	// 3. Store token via SetToken

	return nil, fmt.Errorf("OAuth2 token exchange not yet implemented")
}

// RefreshToken attempts to refresh an expired token
func (m *McpOAuthManager) RefreshToken(serverName string, config *OAuthConfig) (*OAuthToken, error) {
	token, ok := m.GetToken(serverName)
	if !ok || token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available for %s", serverName)
	}

	// TODO: Implement actual token refresh
	// POST to config.TokenURL with grant_type=refresh_token

	return nil, fmt.Errorf("OAuth2 token refresh not yet implemented")
}
