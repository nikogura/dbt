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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nikogura/kubectl-ssh-oidc/pkg/kubectl"
	"github.com/pkg/errors"
)

const debugEnvValue = "true"

// OIDCClientConfig holds client-side OIDC configuration for RFC 8693 token exchange.
type OIDCClientConfig struct {
	IssuerURL        string `json:"issuerUrl,omitempty"`        // OIDC issuer URL for token exchange
	OIDCAudience     string `json:"oidcAudience,omitempty"`     // Target audience for OIDC tokens
	OIDCClientID     string `json:"oidcClientId,omitempty"`     // OAuth2 client ID for token exchange
	OIDCClientSecret string `json:"oidcClientSecret,omitempty"` // OAuth2 client secret for token exchange
	OIDCUsername     string `json:"oidcUsername,omitempty"`     // Username for OIDC (defaults to pubkey username)
	ConnectorID      string `json:"connectorId,omitempty"`      // Connector ID for providers that support it (e.g., "ssh" for Dex)
}

// OIDCClient handles SSH-to-OIDC token exchange via RFC 8693.
type OIDCClient struct {
	IssuerURL    string
	Audience     string
	ClientID     string
	ClientSecret string
	Username     string
	ConnectorID  string
	tokenCache   *tokenCache
}

// tokenCache caches OIDC tokens to avoid repeated exchanges.
type tokenCache struct {
	token     string
	expiresAt time.Time
	mu        sync.RWMutex
}

// DexTokenResponse represents the response from Dex token endpoint.
type DexTokenResponse struct {
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
		IssuerURL:    strings.TrimSuffix(config.IssuerURL, "/"),
		Audience:     config.OIDCAudience,
		ClientID:     config.OIDCClientID,
		ClientSecret: config.OIDCClientSecret,
		Username:     config.OIDCUsername,
		ConnectorID:  config.ConnectorID,
		tokenCache:   &tokenCache{},
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

// exchangeToken performs the SSH-to-OIDC token exchange using kubectl-ssh-oidc.
func (c *OIDCClient) exchangeToken(_ context.Context) (token string, expiresIn int, err error) {
	// Create kubectl-ssh-oidc config for JWT creation
	kubectlConfig := &kubectl.Config{
		DexURL:         c.IssuerURL,
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		DexInstanceID:  c.IssuerURL, // SSH JWT audience = Dex instance URL
		TargetAudience: c.Audience,  // Final OIDC token audience
		Username:       c.Username,
		UseAgent:       true,
	}

	// Create SSH-signed JWT using kubectl-ssh-oidc (this handles key iteration and Dex exchange)
	sshJWT, jwtErr := kubectl.CreateSSHSignedJWT(kubectlConfig)
	if jwtErr != nil {
		err = errors.Wrap(jwtErr, "failed to create SSH-signed JWT")
		return token, expiresIn, err
	}

	// Debug logging
	if os.Getenv("DBT_DEBUG") == debugEnvValue {
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: kubectl-ssh-oidc created JWT successfully\n")
	}

	// Exchange JWT for OIDC token with our custom exchange function
	// (to support dbt-specific audience handling)
	tokenResp, exchangeErr := c.exchangeJWTForOIDC(sshJWT)
	if exchangeErr != nil {
		err = errors.Wrap(exchangeErr, "failed to exchange JWT for OIDC token")
		return token, expiresIn, err
	}

	// Success - prefer ID token, fall back to access token
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

// exchangeJWTForOIDC exchanges an SSH-signed JWT for an OIDC token from Dex.
func (c *OIDCClient) exchangeJWTForOIDC(sshJWT string) (tokenResp *DexTokenResponse, err error) {
	tokenURL := c.IssuerURL + "/token"

	// Prepare OAuth2 Token Exchange request
	formData := url.Values{
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token_type":   {"urn:ietf:params:oauth:token-type:access_token"},
		"subject_token":        {sshJWT},
		"requested_token_type": {"urn:ietf:params:oauth:token-type:id_token"},
		"scope":                {"openid email groups profile"},
		"connector_id":         {c.ConnectorID},
		"client_id":            {c.ClientID},
	}

	// Add audience parameter
	if c.Audience != "" {
		formData.Set("audience", c.Audience)
	}

	// Add client secret if configured
	if c.ClientSecret != "" {
		formData.Set("client_secret", c.ClientSecret)
	}

	// Debug logging
	if os.Getenv("DBT_DEBUG") == debugEnvValue {
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: Token URL: %s\n", tokenURL)
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: client_id: %s\n", c.ClientID)
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: connector_id: %s\n", c.ConnectorID)
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: audience: %s\n", c.Audience)
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: client_secret set: %t\n", c.ClientSecret != "")
	}

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if reqErr != nil {
		err = errors.Wrap(reqErr, "failed to create token request")
		return tokenResp, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = errors.Wrap(respErr, "failed to exchange with Dex")
		return tokenResp, err
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = errors.Wrap(readErr, "failed to read token response")
		return tokenResp, err
	}

	// Debug: log Dex response
	if os.Getenv("DBT_DEBUG") == debugEnvValue {
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: Dex response status: %d\n", resp.StatusCode)
		fmt.Fprintf(os.Stderr, "DBT_DEBUG: Dex response body: %s\n", string(respBody))
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("SSH authentication failed (%d): %s", resp.StatusCode, string(respBody))
		return tokenResp, err
	}

	tokenResp = &DexTokenResponse{}
	unmarshalErr := json.Unmarshal(respBody, tokenResp)
	if unmarshalErr != nil {
		err = errors.Wrap(unmarshalErr, "failed to parse token response")
		return tokenResp, err
	}

	return tokenResp, err
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
