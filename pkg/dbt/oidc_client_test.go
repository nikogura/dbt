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

//nolint:testifylint // test file - assertion style acceptable
package dbt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOIDCClientConfig tests OIDC client configuration.
func TestOIDCClientConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := &OIDCClientConfig{
			IssuerURL:    "https://oidc.example.com",
			OIDCAudience: "dbt-server",
			OIDCUsername: "testuser",
			ConnectorID:  "ssh",
		}

		client, err := NewOIDCClient(config)
		require.NoError(t, err)
		assert.NotNil(t, client)
		assert.Equal(t, "https://oidc.example.com", client.IssuerURL)
		assert.Equal(t, "dbt-server", client.Audience)
		assert.Equal(t, "testuser", client.Username)
		assert.Equal(t, "ssh", client.ConnectorID)
	})

	t.Run("missing issuer url", func(t *testing.T) {
		config := &OIDCClientConfig{
			OIDCAudience: "dbt-server",
		}

		client, err := NewOIDCClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "issuerUrl is required")
	})

	t.Run("missing audience", func(t *testing.T) {
		config := &OIDCClientConfig{
			IssuerURL: "https://oidc.example.com",
		}

		client, err := NewOIDCClient(config)
		assert.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "oidcAudience is required")
	})

	t.Run("trailing slash in issuer url is trimmed", func(t *testing.T) {
		config := &OIDCClientConfig{
			IssuerURL:    "https://oidc.example.com/",
			OIDCAudience: "dbt-server",
		}

		client, err := NewOIDCClient(config)
		require.NoError(t, err)
		assert.Equal(t, "https://oidc.example.com", client.IssuerURL)
	})

	t.Run("connector id is optional", func(t *testing.T) {
		config := &OIDCClientConfig{
			IssuerURL:    "https://oidc.example.com",
			OIDCAudience: "dbt-server",
		}

		client, err := NewOIDCClient(config)
		require.NoError(t, err)
		assert.Empty(t, client.ConnectorID)
	})
}

// TestTokenCache tests the token caching functionality.
func TestTokenCache(t *testing.T) {
	t.Run("cache miss on empty cache", func(t *testing.T) {
		cache := &tokenCache{}
		token, valid := cache.get()
		assert.False(t, valid)
		assert.Empty(t, token)
	})

	t.Run("cache hit after set", func(t *testing.T) {
		cache := &tokenCache{}
		cache.set("test-token", time.Hour)

		token, valid := cache.get()
		assert.True(t, valid)
		assert.Equal(t, "test-token", token)
	})

	t.Run("cache miss when expired", func(t *testing.T) {
		cache := &tokenCache{}
		// Set token that expires in 10 seconds (within the 30 second buffer)
		cache.set("test-token", 10*time.Second)

		token, valid := cache.get()
		assert.False(t, valid)
		assert.Empty(t, token)
	})

	t.Run("cache miss when about to expire", func(t *testing.T) {
		cache := &tokenCache{}
		// Set token that expires in 29 seconds (within the 30 second buffer)
		cache.set("test-token", 29*time.Second)

		token, valid := cache.get()
		assert.False(t, valid)
		assert.Empty(t, token)
	})

	t.Run("cache hit when token has enough time left", func(t *testing.T) {
		cache := &tokenCache{}
		// Set token that expires in 60 seconds (more than 30 second buffer)
		cache.set("test-token", 60*time.Second)

		token, valid := cache.get()
		assert.True(t, valid)
		assert.Equal(t, "test-token", token)
	})
}

// TestTokenResponse tests the OIDC token response parsing.
func TestTokenResponse(t *testing.T) {
	t.Run("parse with id_token", func(t *testing.T) {
		jsonResp := `{
			"access_token": "access-token-123",
			"token_type": "Bearer",
			"expires_in": 3600,
			"id_token": "id-token-456"
		}`

		var resp TokenResponse
		err := json.Unmarshal([]byte(jsonResp), &resp)
		require.NoError(t, err)

		assert.Equal(t, "access-token-123", resp.AccessToken)
		assert.Equal(t, "Bearer", resp.TokenType)
		assert.Equal(t, 3600, resp.ExpiresIn)
		assert.Equal(t, "id-token-456", resp.IDToken)
	})

	t.Run("parse without id_token", func(t *testing.T) {
		jsonResp := `{
			"access_token": "access-token-123",
			"token_type": "Bearer",
			"expires_in": 3600
		}`

		var resp TokenResponse
		err := json.Unmarshal([]byte(jsonResp), &resp)
		require.NoError(t, err)

		assert.Equal(t, "access-token-123", resp.AccessToken)
		assert.Empty(t, resp.IDToken)
	})
}

// mockOIDCServer creates a mock OIDC server for token exchange testing.
func mockOIDCServer(t *testing.T, handler http.HandlerFunc) (server *httptest.Server) {
	t.Helper()
	server = httptest.NewServer(handler)
	return server
}

// TestOIDCClientTokenExchangeErrors tests error handling in token exchange.
func TestOIDCClientTokenExchangeErrors(t *testing.T) {
	t.Run("oidc provider returns error or no ssh-agent", func(t *testing.T) {
		server := mockOIDCServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"error": "invalid_grant", "error_description": "invalid token"}`))
			if err != nil {
				t.Errorf("Failed to write response: %v", err)
			}
		})
		defer server.Close()

		config := &OIDCClientConfig{
			IssuerURL:    server.URL,
			OIDCAudience: "dbt-server",
			OIDCUsername: "testuser",
		}

		client, err := NewOIDCClient(config)
		require.NoError(t, err)

		_, err = client.GetToken(context.Background())
		// In test environment without SSH agent, we'll get an SSH signing error.
		// In production with SSH agent but bad OIDC response, we'd get token exchange error.
		// Either way, we expect an error.
		assert.Error(t, err)
	})

	t.Run("oidc server unreachable", func(t *testing.T) {
		config := &OIDCClientConfig{
			IssuerURL:    "http://localhost:1", // Port 1 is typically not in use
			OIDCAudience: "dbt-server",
			OIDCUsername: "testuser",
		}

		client, err := NewOIDCClient(config)
		require.NoError(t, err)

		_, err = client.GetToken(context.Background())
		assert.Error(t, err)
	})
}

// TestClientConfigFromJSON tests JSON deserialization of client config.
func TestClientConfigFromJSON(t *testing.T) {
	jsonConfig := `{
		"dbt": {
			"repository": "https://dbt.example.com",
			"truststore": "https://dbt.example.com/truststore"
		},
		"tools": {
			"repository": "https://dbt.example.com/tools"
		},
		"authType": "oidc",
		"issuerUrl": "https://oidc.example.com",
		"oidcAudience": "dbt-server",
		"connectorId": "ssh",
		"username": "testuser"
	}`

	var config Config
	err := json.Unmarshal([]byte(jsonConfig), &config)
	require.NoError(t, err)

	assert.Equal(t, "oidc", config.AuthType)
	assert.Equal(t, "https://oidc.example.com", config.IssuerURL)
	assert.Equal(t, "dbt-server", config.OIDCAudience)
	assert.Equal(t, "ssh", config.ConnectorID)
	assert.Equal(t, "testuser", config.Username)
}

// TestMixedAuthConfigClient tests config with both OIDC and legacy auth options.
func TestMixedAuthConfigClient(t *testing.T) {
	t.Run("oidc takes precedence when configured", func(t *testing.T) {
		jsonConfig := `{
			"dbt": {
				"repository": "https://dbt.example.com",
				"truststore": "https://dbt.example.com/truststore"
			},
			"tools": {
				"repository": "https://dbt.example.com/tools"
			},
			"authType": "oidc",
			"issuerUrl": "https://oidc.example.com",
			"oidcAudience": "dbt-server",
			"pubkey": "ssh-rsa AAAAB...",
			"username": "testuser"
		}`

		var config Config
		err := json.Unmarshal([]byte(jsonConfig), &config)
		require.NoError(t, err)

		// When authType is "oidc", OIDC should be used even if pubkey is present.
		assert.Equal(t, "oidc", config.AuthType)
		assert.NotEmpty(t, config.Pubkey) // Pubkey is still present but should be ignored
	})

	t.Run("legacy auth when authType not set", func(t *testing.T) {
		jsonConfig := `{
			"dbt": {
				"repository": "https://dbt.example.com",
				"truststore": "https://dbt.example.com/truststore"
			},
			"tools": {
				"repository": "https://dbt.example.com/tools"
			},
			"pubkey": "ssh-rsa AAAAB...",
			"username": "testuser"
		}`

		var config Config
		err := json.Unmarshal([]byte(jsonConfig), &config)
		require.NoError(t, err)

		// When authType is empty, legacy SSH-agent auth should be used.
		assert.Empty(t, config.AuthType)
		assert.NotEmpty(t, config.Pubkey)
	})
}

// TestOIDCClientConfigSerialization tests round-trip JSON serialization.
func TestOIDCClientConfigSerialization(t *testing.T) {
	original := Config{
		Dbt: DbtConfig{
			Repo:       "https://dbt.example.com",
			TrustStore: "https://dbt.example.com/truststore",
		},
		Tools: ToolsConfig{
			Repo: "https://dbt.example.com/tools",
		},
		AuthType:     "oidc",
		IssuerURL:    "https://oidc.example.com",
		OIDCAudience: "dbt-server",
		ConnectorID:  "ssh",
		Username:     "testuser",
	}

	// Serialize
	data, err := json.Marshal(original)
	require.NoError(t, err)

	// Deserialize
	var restored Config
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify
	assert.Equal(t, original.AuthType, restored.AuthType)
	assert.Equal(t, original.IssuerURL, restored.IssuerURL)
	assert.Equal(t, original.OIDCAudience, restored.OIDCAudience)
	assert.Equal(t, original.ConnectorID, restored.ConnectorID)
	assert.Equal(t, original.Username, restored.Username)
}
