// Package auth provides OAuth authentication for Z.AI
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
)

const (
	// Z.AI OAuth endpoints
	AuthURL  = "https://open.bigmodel.cn/oauth/authorize"
	TokenURL = "https://open.bigmodel.cn/oauth/token"
)

// Config holds authentication configuration
type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
}

// TokenStore stores and retrieves tokens
type TokenStore struct {
	path string
}

// Token represents a stored token
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// NewTokenStore creates a new token store
func NewTokenStore() (*TokenStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	
	path := filepath.Join(home, ".golem", "auth.json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	
	return &TokenStore{path: path}, nil
}

// Save saves a token
func (s *TokenStore) Save(token *Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// Load loads a token
func (s *TokenStore) Load() (*Token, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	
	return &token, nil
}

// IsValid checks if token is valid
func (t *Token) IsValid() bool {
	return t.AccessToken != "" && time.Now().Before(t.ExpiresAt)
}

// OAuthClient handles OAuth flow
type OAuthClient struct {
	config     *oauth2.Config
	tokenStore *TokenStore
}

// NewOAuthClient creates a new OAuth client
func NewOAuthClient(cfg Config) (*OAuthClient, error) {
	store, err := NewTokenStore()
	if err != nil {
		return nil, err
	}
	
	oauthConfig := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  AuthURL,
			TokenURL: TokenURL,
		},
		Scopes: []string{"openid", "api"},
	}
	
	return &OAuthClient{
		config:     oauthConfig,
		tokenStore: store,
	}, nil
}

// GetAuthURL returns the OAuth authorization URL
func (c *OAuthClient) GetAuthURL(state string) string {
	return c.config.AuthCodeURL(state)
}

// Exchange exchanges code for token
func (c *OAuthClient) Exchange(ctx context.Context, code string) (*Token, error) {
	tok, err := c.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	
	token := &Token{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		TokenType:    tok.TokenType,
		ExpiresAt:    tok.Expiry,
	}
	
	if err := c.tokenStore.Save(token); err != nil {
		return nil, fmt.Errorf("save token: %w", err)
	}
	
	return token, nil
}

// GetToken returns a valid token, refreshing if needed
func (c *OAuthClient) GetToken() (*Token, error) {
	token, err := c.tokenStore.Load()
	if err != nil {
		return nil, fmt.Errorf("no stored token: %w", err)
	}
	
	if token.IsValid() {
		return token, nil
	}
	
	// Refresh token
	if token.RefreshToken != "" {
		newTok, err := c.config.TokenSource(context.Background(), &oauth2.Token{
			RefreshToken: token.RefreshToken,
		}).Token()
		if err == nil {
			token = &Token{
				AccessToken:  newTok.AccessToken,
				RefreshToken: newTok.RefreshToken,
				TokenType:    newTok.TokenType,
				ExpiresAt:    newTok.Expiry,
			}
			c.tokenStore.Save(token)
			return token, nil
		}
	}
	
	return nil, fmt.Errorf("token expired and refresh failed")
}

// StartLocalServer starts a local server for OAuth callback
func (c *OAuthClient) StartLocalServer(port int) (<-chan string, error) {
	codeChan := make(chan string, 1)
	
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code != "" {
			codeChan <- code
			fmt.Fprintf(w, `
				<html>
				<body style="background:#1a1a2e;color:#fff;font-family:system-ui;display:flex;align-items:center;justify-content:center;height:100vh;margin:0">
				<div style="text-align:center">
					<h1>✅ Authentication Successful</h1>
					<p>You can close this window and return to Golem.</p>
				</div>
				</body>
				</html>
			`)
		} else {
			fmt.Fprintf(w, "Error: no code received")
		}
	})
	
	go func() {
		http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	}()
	
	return codeChan, nil
}
