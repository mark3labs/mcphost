package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// OAuthClient handles OAuth authentication with Anthropic
type OAuthClient struct {
	ClientID     string
	AuthorizeURL string
	TokenURL     string
	RedirectURI  string
	Scopes       string
}

// AuthData contains authorization URL and PKCE verifier
type AuthData struct {
	URL      string
	Verifier string
}

// NewOAuthClient creates a new OAuth client with Anthropic configuration
func NewOAuthClient() *OAuthClient {
	return &OAuthClient{
		// OAuth client ID is public by design for CLI applications (OAuth public clients).
		// Security is provided by PKCE flow, not by keeping the client ID secret.
		// This follows the same pattern as GitHub CLI, Google Cloud SDK, and other major CLI tools.
		ClientID:     "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
		AuthorizeURL: "https://claude.ai/oauth/authorize",
		TokenURL:     "https://console.anthropic.com/v1/oauth/token",
		RedirectURI:  "https://console.anthropic.com/oauth/code/callback",
		Scopes:       "org:create_api_key user:profile user:inference",
	}
}

// GeneratePKCE generates PKCE verifier and challenge for OAuth flow
func GeneratePKCE() (verifier, challenge string, err error) {
	// Generate 32 bytes of random data
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode verifier as base64url without padding
	verifier = base64.RawURLEncoding.EncodeToString(verifierBytes)

	// Generate challenge by SHA256 hashing the verifier
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// GetAuthorizationURL generates the authorization URL with PKCE parameters
func (c *OAuthClient) GetAuthorizationURL() (*AuthData, error) {
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	params := url.Values{
		"code":                  {"true"},
		"client_id":             {c.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {c.RedirectURI},
		"scope":                 {c.Scopes},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {verifier}, // Using verifier as state (following Python impl)
	}

	authURL := fmt.Sprintf("%s?%s", c.AuthorizeURL, params.Encode())

	return &AuthData{
		URL:      authURL,
		Verifier: verifier,
	}, nil
}

// ExchangeCode exchanges an authorization code for tokens
func (c *OAuthClient) ExchangeCode(code, verifier string) (*AnthropicCredentials, error) {
	// Parse code and state
	parsedCode, parsedState := c.parseCodeAndState(code)

	// Build request body
	reqBody := map[string]interface{}{
		"code":          parsedCode,
		"grant_type":    "authorization_code",
		"client_id":     c.ClientID,
		"redirect_uri":  c.RedirectURI,
		"code_verifier": verifier,
	}

	// Include state if present
	if parsedState != "" {
		reqBody["state"] = parsedState
	}

	// Make request
	return c.makeTokenRequest(reqBody)
}

// RefreshToken refreshes an access token using a refresh token
func (c *OAuthClient) RefreshToken(refreshToken string) (*AnthropicCredentials, error) {
	reqBody := map[string]interface{}{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     c.ClientID,
	}

	return c.makeTokenRequest(reqBody)
}

// makeTokenRequest makes a token request to the OAuth server
func (c *OAuthClient) makeTokenRequest(body map[string]interface{}) (*AnthropicCredentials, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", c.TokenURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorResp); err == nil {
			return nil, fmt.Errorf("token request failed: %v", errorResp)
		}
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &AnthropicCredentials{
		Type:         "oauth",
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
		ExpiresAt:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
		CreatedAt:    time.Now(),
	}, nil
}

// parseCodeAndState parses the authorization code and state from the callback
func (c *OAuthClient) parseCodeAndState(code string) (parsedCode, parsedState string) {
	splits := strings.Split(code, "#")
	parsedCode = splits[0]
	if len(splits) > 1 {
		parsedState = splits[1]
	}
	return
}

// SetOAuthCredentials stores OAuth credentials
func (cm *CredentialManager) SetOAuthCredentials(creds *AnthropicCredentials) error {
	store, err := cm.LoadCredentials()
	if err != nil {
		return err
	}

	store.Anthropic = creds
	return cm.SaveCredentials(store)
}

// GetValidAccessToken returns a valid access token, refreshing if necessary
func (cm *CredentialManager) GetValidAccessToken() (string, error) {
	creds, err := cm.GetAnthropicCredentials()
	if err != nil {
		return "", err
	}

	if creds == nil {
		return "", fmt.Errorf("no credentials found")
	}

	// For API key auth, return the API key
	if creds.Type == "api_key" {
		return creds.APIKey, nil
	}

	// For OAuth, check if token needs refresh
	if creds.Type == "oauth" {
		if creds.NeedsRefresh() {
			// Refresh the token
			client := NewOAuthClient()
			newCreds, err := client.RefreshToken(creds.RefreshToken)
			if err != nil {
				return "", fmt.Errorf("failed to refresh token: %w", err)
			}

			// Update stored credentials
			if err := cm.SetOAuthCredentials(newCreds); err != nil {
				return "", fmt.Errorf("failed to save refreshed token: %w", err)
			}

			return newCreds.AccessToken, nil
		}

		return creds.AccessToken, nil
	}

	return "", fmt.Errorf("unknown credential type: %s", creds.Type)
}
