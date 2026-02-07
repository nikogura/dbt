// Copyright 2025 Nik Ogura <nik.ogura@gmail.com>
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

package installer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/nikogura/kubectl-ssh-oidc/pkg/kubectl"
)

// AuthClient handles OIDC authentication for the installer.
type AuthClient struct {
	config      *Config
	username    string
	cachedToken string
}

// NewAuthClient creates a new authentication client.
func NewAuthClient(config *Config, username string) (client *AuthClient) {
	client = &AuthClient{
		config:   config,
		username: username,
	}
	return client
}

// GetToken retrieves an OIDC token using the appropriate method.
// For SSH-OIDC (connectorID == "ssh"), uses SSH keys via jwt-ssh-agent-go.
// Otherwise, uses the device code flow (browser-based).
func (a *AuthClient) GetToken(ctx context.Context) (token string, err error) {
	if a.cachedToken != "" {
		token = a.cachedToken
		return token, err
	}

	if a.config.ConnectorID == connectorSSH {
		token, err = a.getSSHOIDCToken(ctx)
	} else {
		token, err = a.getDeviceFlowToken(ctx)
	}

	if err == nil {
		a.cachedToken = token
	}

	return token, err
}

// getSSHOIDCToken performs SSH-OIDC token exchange using kubectl-ssh-oidc.
func (a *AuthClient) getSSHOIDCToken(_ context.Context) (token string, err error) {
	fmt.Println("Authenticating via SSH-OIDC...")

	// Configure kubectl-ssh-oidc
	kubectlConfig := &kubectl.Config{
		DexURL:         a.config.IssuerURL,
		ClientID:       a.config.OIDCClientID,
		ClientSecret:   a.config.OIDCClientSecret,
		DexInstanceID:  a.config.IssuerURL,
		TargetAudience: a.config.OIDCAudience,
		Username:       a.username,
		UseAgent:       true,
	}

	// Create SSH-signed JWT (this handles key iteration internally)
	var jwtErr error
	token, jwtErr = kubectl.CreateSSHSignedJWT(kubectlConfig)
	if jwtErr != nil {
		err = fmt.Errorf("failed to create SSH-signed JWT: %w\n\nMake sure your SSH key is loaded: ssh-add -l", jwtErr)
		return token, err
	}

	// Exchange with Dex for OIDC token
	tokenResp, exchangeErr := a.exchangeJWTForOIDC(token)
	if exchangeErr != nil {
		err = fmt.Errorf("SSH-OIDC token exchange failed: %w\n\nCheck that your SSH key is authorized in Dex", exchangeErr)
		return token, err
	}

	// Prefer ID token, fall back to access token
	if tokenResp.IDToken != "" {
		token = tokenResp.IDToken
	} else {
		token = tokenResp.AccessToken
	}

	fmt.Println("  SSH-OIDC authentication successful")
	return token, err
}

// TokenResponse represents the response from the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

// exchangeJWTForOIDC exchanges an SSH-signed JWT for an OIDC token.
func (a *AuthClient) exchangeJWTForOIDC(sshJWT string) (tokenResp *TokenResponse, err error) {
	tokenURL := strings.TrimSuffix(a.config.IssuerURL, "/") + "/token"

	formData := url.Values{
		"grant_type":           {"urn:ietf:params:oauth:grant-type:token-exchange"},
		"subject_token_type":   {"urn:ietf:params:oauth:token-type:access_token"},
		"subject_token":        {sshJWT},
		"requested_token_type": {"urn:ietf:params:oauth:token-type:id_token"},
		"scope":                {"openid email groups profile"},
		"connector_id":         {a.config.ConnectorID},
		"client_id":            {a.config.OIDCClientID},
	}

	if a.config.OIDCAudience != "" {
		formData.Set("audience", a.config.OIDCAudience)
	}
	if a.config.OIDCClientSecret != "" {
		formData.Set("client_secret", a.config.OIDCClientSecret)
	}

	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodPost, tokenURL, strings.NewReader(formData.Encode()))
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return tokenResp, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("request failed: %w", respErr)
		return tokenResp, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = fmt.Errorf("failed to read response: %w", readErr)
		return tokenResp, err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
		return tokenResp, err
	}

	tokenResp = &TokenResponse{}
	unmarshalErr := json.Unmarshal(body, tokenResp)
	if unmarshalErr != nil {
		err = fmt.Errorf("failed to parse response: %w", unmarshalErr)
		return tokenResp, err
	}

	return tokenResp, err
}

// DeviceCodeResponse represents the device authorization response.
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// OIDCDiscovery represents the OIDC discovery document.
type OIDCDiscovery struct {
	TokenEndpoint               string `json:"token_endpoint"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
}

// getDeviceFlowToken performs OIDC device code flow authentication.
func (a *AuthClient) getDeviceFlowToken(ctx context.Context) (token string, err error) {
	fmt.Println("Authenticating via OIDC device flow...")

	// Discover OIDC endpoints
	discovery, discErr := a.discoverOIDC()
	if discErr != nil {
		err = fmt.Errorf("OIDC discovery failed: %w", discErr)
		return token, err
	}

	if discovery.DeviceAuthorizationEndpoint == "" {
		err = errors.New("OIDC provider does not support device code flow")
		return token, err
	}

	// Request device code
	deviceResp, deviceErr := a.requestDeviceCode(discovery.DeviceAuthorizationEndpoint)
	if deviceErr != nil {
		err = fmt.Errorf("failed to get device code: %w", deviceErr)
		return token, err
	}

	// Display instructions to user
	fmt.Println()
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("    Open this URL in your browser:\n")
	fmt.Printf("    %s\n", deviceResp.VerificationURI)
	fmt.Println()
	fmt.Printf("    Enter code: %s\n", deviceResp.UserCode)
	fmt.Println("  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Try to open browser
	openBrowser(deviceResp.VerificationURI)

	// Poll for token
	interval := deviceResp.Interval
	if interval == 0 {
		interval = 5
	}

	deadline := time.Now().Add(time.Duration(deviceResp.ExpiresIn) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return token, err
		case <-time.After(time.Duration(interval) * time.Second):
		}

		var pollErr error
		token, pollErr = a.pollForToken(discovery.TokenEndpoint, deviceResp.DeviceCode)
		if pollErr == nil {
			fmt.Println("  Authentication successful")
			return token, err
		}

		// Check if we should keep polling
		if !strings.Contains(pollErr.Error(), "authorization_pending") &&
			!strings.Contains(pollErr.Error(), "slow_down") {
			err = pollErr
			return token, err
		}
	}

	err = errors.New("authentication timed out")
	return token, err
}

// discoverOIDC fetches the OIDC discovery document.
func (a *AuthClient) discoverOIDC() (discovery *OIDCDiscovery, err error) {
	discoveryURL := strings.TrimSuffix(a.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return discovery, err
	}

	client := &http.Client{}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("failed to fetch discovery: %w", respErr)
		return discovery, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = fmt.Errorf("failed to read discovery: %w", readErr)
		return discovery, err
	}

	discovery = &OIDCDiscovery{}
	unmarshalErr := json.Unmarshal(body, discovery)
	if unmarshalErr != nil {
		err = fmt.Errorf("failed to parse discovery: %w", unmarshalErr)
		return discovery, err
	}

	return discovery, err
}

// requestDeviceCode requests a device code from the authorization server.
func (a *AuthClient) requestDeviceCode(endpoint string) (deviceResp *DeviceCodeResponse, err error) {
	formData := url.Values{
		"client_id": {a.config.OIDCClientID},
		"scope":     {"openid profile email groups offline_access"},
	}

	if a.config.OIDCAudience != "" {
		formData.Set("audience", a.config.OIDCAudience)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(formData.Encode()))
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return deviceResp, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("request failed: %w", respErr)
		return deviceResp, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = fmt.Errorf("failed to read response: %w", readErr)
		return deviceResp, err
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("device code request failed (%d): %s", resp.StatusCode, string(body))
		return deviceResp, err
	}

	deviceResp = &DeviceCodeResponse{}
	unmarshalErr := json.Unmarshal(body, deviceResp)
	if unmarshalErr != nil {
		err = fmt.Errorf("failed to parse response: %w", unmarshalErr)
		return deviceResp, err
	}

	return deviceResp, err
}

// pollForToken polls the token endpoint for the authorization result.
func (a *AuthClient) pollForToken(tokenEndpoint, deviceCode string) (token string, err error) {
	formData := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {a.config.OIDCClientID},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(formData.Encode()))
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return token, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("request failed: %w", respErr)
		return token, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = fmt.Errorf("failed to read response: %w", readErr)
		return token, err
	}

	// Check for error response
	var errorResp struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &errorResp)

	if errorResp.Error != "" {
		err = fmt.Errorf("%s: %s", errorResp.Error, errorResp.ErrorDescription)
		return token, err
	}

	var tokenResp TokenResponse
	unmarshalErr := json.Unmarshal(body, &tokenResp)
	if unmarshalErr != nil {
		err = fmt.Errorf("failed to parse response: %w", unmarshalErr)
		return token, err
	}

	// Prefer ID token
	if tokenResp.IDToken != "" {
		token = tokenResp.IDToken
	} else {
		token = tokenResp.AccessToken
	}

	return token, err
}

// openBrowser attempts to open a URL in the default browser.
func openBrowser(browserURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", browserURL)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", browserURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", browserURL)
	default:
		return
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Start()
}
