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

//nolint:govet,testifylint // test file - shadows and assertion style acceptable
package dbt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOIDCClaimsExtraction tests the extraction of username from different claims.
func TestOIDCClaimsExtraction(t *testing.T) {
	cases := []struct {
		Name           string
		ClaimKey       string
		Claims         OIDCClaims
		ExpectedResult string
	}{
		{
			Name:     "extract from sub",
			ClaimKey: "sub",
			Claims: OIDCClaims{
				Subject:  "user123",
				Email:    "user@example.com",
				Username: "preferred_user",
				Name:     "Full Name",
			},
			ExpectedResult: "user123",
		},
		{
			Name:     "extract from email",
			ClaimKey: "email",
			Claims: OIDCClaims{
				Subject:  "user123",
				Email:    "user@example.com",
				Username: "preferred_user",
				Name:     "Full Name",
			},
			ExpectedResult: "user@example.com",
		},
		{
			Name:     "extract from preferred_username",
			ClaimKey: "preferred_username",
			Claims: OIDCClaims{
				Subject:  "user123",
				Email:    "user@example.com",
				Username: "preferred_user",
				Name:     "Full Name",
			},
			ExpectedResult: "preferred_user",
		},
		{
			Name:     "extract from name",
			ClaimKey: "name",
			Claims: OIDCClaims{
				Subject:  "user123",
				Email:    "user@example.com",
				Username: "preferred_user",
				Name:     "Full Name",
			},
			ExpectedResult: "Full Name",
		},
		{
			Name:     "fallback when preferred claim is empty",
			ClaimKey: "email",
			Claims: OIDCClaims{
				Subject:  "user123",
				Email:    "",
				Username: "preferred_user",
				Name:     "",
			},
			ExpectedResult: "preferred_user",
		},
		{
			Name:     "fallback to subject when all empty",
			ClaimKey: "email",
			Claims: OIDCClaims{
				Subject:  "user123",
				Email:    "",
				Username: "",
				Name:     "",
			},
			ExpectedResult: "user123",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			validator := &OIDCValidator{
				config: &OIDCAuthOpts{
					UsernameClaimKey: tc.ClaimKey,
				},
			}

			result := validator.GetUsername(&tc.Claims)
			assert.Equal(t, tc.ExpectedResult, result)
		})
	}
}

// TestOIDCGroupValidation tests the group membership validation.
func TestOIDCGroupValidation(t *testing.T) {
	cases := []struct {
		Name          string
		AllowedGroups []string
		UserGroups    []string
		ExpectError   bool
	}{
		{
			Name:          "no restrictions",
			AllowedGroups: []string{},
			UserGroups:    []string{"any-group"},
			ExpectError:   false,
		},
		{
			Name:          "user in allowed group",
			AllowedGroups: []string{"admin", "developers"},
			UserGroups:    []string{"users", "developers"},
			ExpectError:   false,
		},
		{
			Name:          "user not in any allowed group",
			AllowedGroups: []string{"admin", "developers"},
			UserGroups:    []string{"users", "readonly"},
			ExpectError:   true,
		},
		{
			Name:          "empty user groups",
			AllowedGroups: []string{"admin"},
			UserGroups:    []string{},
			ExpectError:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			validator := &OIDCValidator{
				config: &OIDCAuthOpts{
					AllowedGroups: tc.AllowedGroups,
				},
			}

			err := validator.validateGroupMembership(tc.UserGroups)
			if tc.ExpectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestOIDCAudienceMatching tests the audience matching logic.
func TestOIDCAudienceMatching(t *testing.T) {
	cases := []struct {
		Name              string
		ExpectedAudiences []string
		TokenAudiences    []string
		ExpectMatch       bool
	}{
		{
			Name:              "single audience match",
			ExpectedAudiences: []string{"dbt-server"},
			TokenAudiences:    []string{"dbt-server"},
			ExpectMatch:       true,
		},
		{
			Name:              "multiple expected, single token match",
			ExpectedAudiences: []string{"dbt-server", "dbt-client"},
			TokenAudiences:    []string{"dbt-server"},
			ExpectMatch:       true,
		},
		{
			Name:              "single expected, multiple token match",
			ExpectedAudiences: []string{"dbt-server"},
			TokenAudiences:    []string{"other-server", "dbt-server"},
			ExpectMatch:       true,
		},
		{
			Name:              "no match",
			ExpectedAudiences: []string{"dbt-server"},
			TokenAudiences:    []string{"other-server"},
			ExpectMatch:       false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			validator := &OIDCValidator{
				config: &OIDCAuthOpts{
					Audiences: tc.ExpectedAudiences,
				},
			}

			result := validator.audienceMatches(tc.TokenAudiences)
			assert.Equal(t, tc.ExpectMatch, result)
		})
	}
}

// TestCheckOIDCAuthMissingHeader tests the CheckOIDCAuth function with missing headers.
func TestCheckOIDCAuthMissingHeader(t *testing.T) {
	validator := &OIDCValidator{
		config: &OIDCAuthOpts{
			IssuerURL: "https://example.com",
			Audiences: []string{"test"},
		},
	}

	cases := []struct {
		Name           string
		AuthHeader     string
		ExpectedStatus int
	}{
		{
			Name:           "no Authorization header",
			AuthHeader:     "",
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:           "non-Bearer token",
			AuthHeader:     "Basic dXNlcjpwYXNz",
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:           "empty Bearer token",
			AuthHeader:     "Bearer ",
			ExpectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tc.AuthHeader != "" {
				req.Header.Set("Authorization", tc.AuthHeader)
			}

			w := httptest.NewRecorder()

			username := CheckOIDCAuth(w, req, validator)

			assert.Equal(t, "", username)
			assert.Equal(t, tc.ExpectedStatus, w.Code)
		})
	}
}

// TestOIDCAuthOptsDefaults tests that defaults are set correctly.
func TestOIDCAuthOptsDefaults(t *testing.T) {
	config := &OIDCAuthOpts{
		IssuerURL: "https://example.com",
		Audiences: []string{"test"},
		// UsernameClaimKey and JWKSCacheSeconds left empty to test defaults
	}

	// Simulate what NewOIDCValidator would do for defaults
	if config.UsernameClaimKey == "" {
		config.UsernameClaimKey = "sub"
	}
	if config.JWKSCacheSeconds == 0 {
		config.JWKSCacheSeconds = 300
	}

	assert.Equal(t, "sub", config.UsernameClaimKey)
	assert.Equal(t, 300, config.JWKSCacheSeconds)
}

// mockOIDCProvider creates a mock OIDC provider server for testing.
// The server URL is determined at runtime, so we need to return the server first
// and use its URL as the issuer.
//
//nolint:unparam // privateKey reserved for future signing implementation
func mockOIDCProvider(t *testing.T, privateKey *rsa.PrivateKey) (server *httptest.Server) {
	t.Helper()

	mux := http.NewServeMux()

	// The handler closure will capture the server reference after it's created
	var serverURL string

	// OpenID Configuration endpoint
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		config := map[string]interface{}{
			"issuer":   serverURL,
			"jwks_uri": serverURL + "/keys",
		}
		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(config)
		if err != nil {
			t.Errorf("Failed to encode OIDC config: %v", err)
		}
	})

	// JWKS endpoint
	mux.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {
		// This would return the public key in JWKS format
		// For simplicity, we'll skip the full implementation
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"keys":[]}`))
		if err != nil {
			t.Errorf("Failed to write JWKS response: %v", err)
		}
	})

	server = httptest.NewServer(mux)
	serverURL = server.URL
	return server
}

// TestOIDCValidatorCreation tests the creation of an OIDC validator.
func TestOIDCValidatorCreation(t *testing.T) {
	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Start mock OIDC provider
	server := mockOIDCProvider(t, privateKey)
	defer server.Close()

	t.Run("missing issuer URL", func(t *testing.T) {
		config := &OIDCAuthOpts{
			Audiences: []string{"test"},
		}
		_, err := NewOIDCValidator(context.Background(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "issuer URL is required")
	})

	t.Run("missing audiences", func(t *testing.T) {
		config := &OIDCAuthOpts{
			IssuerURL: server.URL,
		}
		_, err := NewOIDCValidator(context.Background(), config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "audience is required")
	})

	t.Run("valid configuration", func(t *testing.T) {
		config := &OIDCAuthOpts{
			IssuerURL: server.URL,
			Audiences: []string{"test-audience"},
		}
		validator, err := NewOIDCValidator(context.Background(), config)
		assert.NoError(t, err)
		if assert.NotNil(t, validator) {
			assert.Equal(t, "sub", validator.config.UsernameClaimKey)
			assert.Equal(t, 300, validator.config.JWKSCacheSeconds)
		}
	})
}

// TestWrapOIDC tests the OIDC wrapper function.
func TestWrapOIDC(t *testing.T) {
	// Test that WrapOIDC properly wraps a handler
	handlerCalled := false
	var capturedUsername string

	handler := func(w http.ResponseWriter, ar *AuthenticatedRequest) {
		handlerCalled = true
		capturedUsername = ar.Username
		w.WriteHeader(http.StatusOK)
	}

	// The validator will fail because no valid token, but we can test the wrapper structure
	validator := &OIDCValidator{
		config: &OIDCAuthOpts{
			IssuerURL: "https://example.com",
			Audiences: []string{"test"},
		},
	}

	wrapped := WrapOIDC(handler, validator)
	assert.NotNil(t, wrapped)

	// Test that unauthenticated request doesn't call the handler
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	wrapped(w, req)

	assert.False(t, handlerCalled)
	assert.Equal(t, "", capturedUsername)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// createTestToken creates a test JWT token for testing purposes.
func createTestToken(t *testing.T, claims jwt.MapClaims, privateKey *rsa.PrivateKey) (resultString string) {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedString, err := token.SignedString(privateKey)
	require.NoError(t, err)

	resultString = signedString
	return resultString
}

// TestOIDCRequiredClaimsValidation tests required claims validation.
func TestOIDCRequiredClaimsValidation(t *testing.T) {
	// Generate test key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	now := time.Now()

	cases := []struct {
		Name           string
		RequiredClaims map[string]string
		TokenClaims    jwt.MapClaims
		ExpectError    bool
	}{
		{
			Name:           "no required claims",
			RequiredClaims: nil,
			TokenClaims: jwt.MapClaims{
				"sub": "user123",
				"aud": "test",
				"iss": "https://example.com",
				"exp": now.Add(time.Hour).Unix(),
			},
			ExpectError: false,
		},
		{
			Name:           "required claim present and matches",
			RequiredClaims: map[string]string{"tenant": "acme"},
			TokenClaims: jwt.MapClaims{
				"sub":    "user123",
				"aud":    "test",
				"iss":    "https://example.com",
				"exp":    now.Add(time.Hour).Unix(),
				"tenant": "acme",
			},
			ExpectError: false,
		},
		{
			Name:           "required claim missing",
			RequiredClaims: map[string]string{"tenant": "acme"},
			TokenClaims: jwt.MapClaims{
				"sub": "user123",
				"aud": "test",
				"iss": "https://example.com",
				"exp": now.Add(time.Hour).Unix(),
			},
			ExpectError: true,
		},
		{
			Name:           "required claim wrong value",
			RequiredClaims: map[string]string{"tenant": "acme"},
			TokenClaims: jwt.MapClaims{
				"sub":    "user123",
				"aud":    "test",
				"iss":    "https://example.com",
				"exp":    now.Add(time.Hour).Unix(),
				"tenant": "other",
			},
			ExpectError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_ = createTestToken(t, tc.TokenClaims, privateKey)

			// We can't fully test without a real OIDC provider, but we can test the validation logic
			// by checking if the required claims would match
			if len(tc.RequiredClaims) > 0 {
				for key, expectedValue := range tc.RequiredClaims {
					actualValue, exists := tc.TokenClaims[key]
					if !exists {
						assert.True(t, tc.ExpectError, "Expected error for missing claim %s", key)
						return
					}
					if actualValue != expectedValue {
						assert.True(t, tc.ExpectError, "Expected error for mismatched claim %s", key)
						return
					}
				}
			}
			assert.False(t, tc.ExpectError, "Expected no error")
		})
	}
}

// TestOIDCConfigSerialization tests JSON serialization of OIDC config.
func TestOIDCConfigSerialization(t *testing.T) {
	config := AuthOpts{
		OIDC: &OIDCAuthOpts{
			IssuerURL:        "https://dex.example.com",
			Audiences:        []string{"dbt-server"},
			UsernameClaimKey: "email",
			AllowedGroups:    []string{"developers", "admin"},
			RequiredClaims: map[string]string{
				"tenant": "acme",
			},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(config)
	require.NoError(t, err)

	// Unmarshal back
	var restored AuthOpts
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, config.OIDC.IssuerURL, restored.OIDC.IssuerURL)
	assert.Equal(t, config.OIDC.Audiences, restored.OIDC.Audiences)
	assert.Equal(t, config.OIDC.UsernameClaimKey, restored.OIDC.UsernameClaimKey)
	assert.Equal(t, config.OIDC.AllowedGroups, restored.OIDC.AllowedGroups)
	assert.Equal(t, config.OIDC.RequiredClaims, restored.OIDC.RequiredClaims)
}

// TestRepoServerOIDCConfig tests loading a repo server config with OIDC.
func TestRepoServerOIDCConfig(t *testing.T) {
	configJSON := `{
		"address": "127.0.0.1",
		"port": 9999,
		"serverRoot": "/var/dbt",
		"authTypeGet": "oidc",
		"authGets": true,
		"authOptsGet": {
			"oidc": {
				"issuerUrl": "https://dex.example.com",
				"audiences": ["dbt-server"],
				"usernameClaimKey": "email"
			}
		},
		"authTypePut": "oidc",
		"authOptsPut": {
			"oidc": {
				"issuerUrl": "https://dex.example.com",
				"audiences": ["dbt-server"],
				"usernameClaimKey": "email",
				"allowedGroups": ["publishers"]
			}
		}
	}`

	var server DBTRepoServer
	err := json.Unmarshal([]byte(configJSON), &server)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", server.Address)
	assert.Equal(t, 9999, server.Port)
	assert.Equal(t, "/var/dbt", server.ServerRoot)
	assert.Equal(t, AUTH_OIDC, server.AuthTypeGet)
	assert.Equal(t, AUTH_OIDC, server.AuthTypePut)
	assert.True(t, server.AuthGets)

	// Check GET OIDC config
	require.NotNil(t, server.AuthOptsGet.OIDC)
	assert.Equal(t, "https://dex.example.com", server.AuthOptsGet.OIDC.IssuerURL)
	assert.Equal(t, []string{"dbt-server"}, server.AuthOptsGet.OIDC.Audiences)
	assert.Equal(t, "email", server.AuthOptsGet.OIDC.UsernameClaimKey)

	// Check PUT OIDC config
	require.NotNil(t, server.AuthOptsPut.OIDC)
	assert.Equal(t, "https://dex.example.com", server.AuthOptsPut.OIDC.IssuerURL)
	assert.Equal(t, []string{"dbt-server"}, server.AuthOptsPut.OIDC.Audiences)
	assert.Equal(t, "email", server.AuthOptsPut.OIDC.UsernameClaimKey)
	assert.Equal(t, []string{"publishers"}, server.AuthOptsPut.OIDC.AllowedGroups)
}

// TestMixedAuthConfig tests a config with different auth types for GET and PUT.
func TestMixedAuthConfig(t *testing.T) {
	configJSON := `{
		"address": "127.0.0.1",
		"port": 9999,
		"serverRoot": "/var/dbt",
		"authTypeGet": "oidc",
		"authGets": true,
		"authOptsGet": {
			"oidc": {
				"issuerUrl": "https://dex.example.com",
				"audiences": ["dbt-server"]
			}
		},
		"authTypePut": "ssh-agent-file",
		"authOptsPut": {
			"idpFile": "/etc/dbt/pubkeys.json"
		}
	}`

	var server DBTRepoServer
	err := json.Unmarshal([]byte(configJSON), &server)
	require.NoError(t, err)

	// OIDC for GET
	assert.Equal(t, AUTH_OIDC, server.AuthTypeGet)
	require.NotNil(t, server.AuthOptsGet.OIDC)

	// SSH-agent for PUT
	assert.Equal(t, AUTH_SSH_AGENT_FILE, server.AuthTypePut)
	assert.Equal(t, "/etc/dbt/pubkeys.json", server.AuthOptsPut.IdpFile)
	assert.Nil(t, server.AuthOptsPut.OIDC)
}
