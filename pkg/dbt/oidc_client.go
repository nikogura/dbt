// Copyright © 2025 Nik Ogura <nik.ogura@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbt

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/nikogura/kubectl-ssh-oidc/pkg/kubectl"
	"github.com/pkg/errors"
)

// OIDCClientConfig holds client-side OIDC configuration for RFC 8693 token exchange.
type OIDCClientConfig struct {
	IssuerURL    string `json:"issuerUrl,omitempty"`    // OIDC issuer URL for token exchange
	OIDCAudience string `json:"oidcAudience,omitempty"` // Target audience for OIDC tokens
	OIDCClientID string `json:"oidcClientId,omitempty"` // OAuth2 client ID for token exchange
	OIDCUsername string `json:"oidcUsername,omitempty"` // Username for OIDC (defaults to pubkey username)
	ConnectorID  string `json:"connectorId,omitempty"`  // Connector ID for providers that support it (e.g., "ssh" for Dex)
}

// OIDCClient handles SSH-to-OIDC token exchange via RFC 8693.
type OIDCClient struct {
	IssuerURL   string
	Audience    string
	ClientID    string
	Username    string
	ConnectorID string
	tokenCache  *tokenCache
}

// tokenCache caches OIDC tokens to avoid repeated exchanges.
type tokenCache struct {
	token     string
	expiresAt time.Time
	mu        sync.RWMutex
}

// TokenResponse represents the response from an OIDC token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// NewOIDCClient creates a new OIDC client for token exchange.
func NewOIDCClient(config *OIDCClientConfig) (client *OIDCClient, err error) {
	if config.IssuerURL == "" {
		err = errors.New("issuerUrl is required for OIDC authentication")
		return client, err
	}

	if config.OIDCAudience == "" {
		err = errors.New("oidcAudience is required for OIDC authentication")
		return client, err
	}

	client = &OIDCClient{
		IssuerURL:   strings.TrimSuffix(config.IssuerURL, "/"),
		Audience:    config.OIDCAudience,
		ClientID:    config.OIDCClientID,
		Username:    config.OIDCUsername,
		ConnectorID: config.ConnectorID,
		tokenCache:  &tokenCache{},
	}

	return client, err
}

// GetToken retrieves an OIDC token, using cache if valid.
func (c *OIDCClient) GetToken(ctx context.Context) (token string, err error) {
	// Check cache first
	cachedToken, valid := c.tokenCache.get()
	if valid {
		token = cachedToken
		return token, err
	}

	// Perform token exchange
	var expiresIn int
	token, expiresIn, err = c.exchangeToken(ctx)
	if err != nil {
		return token, err
	}

	// Cache the token (with 30 second buffer before expiry)
	c.tokenCache.set(token, time.Duration(expiresIn)*time.Second)

	return token, err
}

// exchangeToken performs the SSH-to-OIDC token exchange via kubectl-ssh-oidc.
func (c *OIDCClient) exchangeToken(_ context.Context) (token string, expiresIn int, err error) {
	// Create kubectl-ssh-oidc config
	config := &kubectl.Config{
		DexURL:         c.IssuerURL,
		Username:       c.Username,
		DexInstanceID:  c.IssuerURL, // SSH JWT audience is the Dex instance
		TargetAudience: c.Audience,  // Final OIDC token audience
		UseAgent:       true,        // Use SSH agent for key discovery
	}

	// If client ID is set, use it
	if c.ClientID != "" {
		config.ClientID = c.ClientID
	}

	// Create SSH-signed JWT using kubectl-ssh-oidc (handles SSH agent key discovery)
	sshJWT, jwtErr := kubectl.CreateSSHSignedJWT(config)
	if jwtErr != nil {
		err = errors.Wrap(jwtErr, "failed to create SSH-signed JWT")
		return token, expiresIn, err
	}

	// Exchange with Dex for OIDC token
	tokenResp, exchangeErr := kubectl.ExchangeWithDex(config, sshJWT)
	if exchangeErr != nil {
		err = errors.Wrap(exchangeErr, "failed to exchange JWT with Dex")
		return token, expiresIn, err
	}

	// Prefer ID token, fall back to access token
	if tokenResp.IDToken != "" {
		token = tokenResp.IDToken
	} else {
		token = tokenResp.AccessToken
	}

	expiresIn = tokenResp.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600 // Default to 1 hour
	}

	return token, expiresIn, err
}

func (tc *tokenCache) get() (token string, valid bool) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	// Consider token invalid 30 seconds before actual expiry
	if time.Now().Before(tc.expiresAt.Add(-30 * time.Second)) {
		token = tc.token
		valid = true
		return token, valid
	}

	token = ""
	valid = false
	return token, valid
}

func (tc *tokenCache) set(token string, expiresIn time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.token = token
	tc.expiresAt = time.Now().Add(expiresIn)
}
