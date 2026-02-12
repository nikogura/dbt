//nolint:govet // test file - shadows acceptable in test setup
package dbt

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAuthTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single type",
			input:    "static-token",
			expected: []string{"static-token"},
		},
		{
			name:     "two types",
			input:    "static-token,oidc",
			expected: []string{"static-token", "oidc"},
		},
		{
			name:     "two types with spaces",
			input:    "static-token , oidc",
			expected: []string{"static-token", "oidc"},
		},
		{
			name:     "three types",
			input:    "static-token,oidc,basic-htpasswd",
			expected: []string{"static-token", "oidc", "basic-htpasswd"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "trailing comma",
			input:    "static-token,",
			expected: []string{"static-token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAuthTypes(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsAuthType(t *testing.T) {
	assert.True(t, containsAuthType("static-token,oidc", "oidc"))
	assert.True(t, containsAuthType("static-token,oidc", "static-token"))
	assert.False(t, containsAuthType("static-token,oidc", "basic-htpasswd"))
	assert.True(t, containsAuthType("oidc", "oidc"))
	assert.False(t, containsAuthType("", "oidc"))
}

func TestResolveStaticToken(t *testing.T) {
	t.Run("config value", func(t *testing.T) {
		opts := AuthOpts{StaticToken: "from-config"}
		token := resolveStaticToken(opts)
		assert.Equal(t, "from-config", token)
	})

	t.Run("env var takes precedence", func(t *testing.T) {
		envVar := "DBT_TEST_RESOLVE_TOKEN"
		t.Setenv(envVar, "from-env")

		opts := AuthOpts{
			StaticToken:    "from-config",
			StaticTokenEnv: envVar,
		}
		token := resolveStaticToken(opts)
		assert.Equal(t, "from-env", token)
	})

	t.Run("falls back to config when env unset", func(t *testing.T) {
		opts := AuthOpts{
			StaticToken:    "from-config",
			StaticTokenEnv: "DBT_TEST_NONEXISTENT_VAR",
		}
		token := resolveStaticToken(opts)
		assert.Equal(t, "from-config", token)
	})
}

func TestTryStaticTokenAuth(t *testing.T) {
	expectedToken := "test-token-abc123"

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/test", nil)
		req.Header.Set("Authorization", "Bearer "+expectedToken)

		username := TryStaticTokenAuth(req, expectedToken)
		assert.Equal(t, AUTH_STATIC_TOKEN, username)
	})

	t.Run("wrong token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/test", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")

		username := TryStaticTokenAuth(req, expectedToken)
		assert.Empty(t, username)
	})

	t.Run("no auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/test", nil)

		username := TryStaticTokenAuth(req, expectedToken)
		assert.Empty(t, username)
	})

	t.Run("basic auth header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/test", nil)
		req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

		username := TryStaticTokenAuth(req, expectedToken)
		assert.Empty(t, username)
	})

	t.Run("empty expected token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/test", nil)
		req.Header.Set("Authorization", "Bearer some-token")

		username := TryStaticTokenAuth(req, "")
		assert.Empty(t, username)
	})
}

func TestMultiAuthStaticTokenAndHtpasswd(t *testing.T) {
	tmpDir := t.TempDir()

	// Generate random static token.
	tokenBytes := make([]byte, 32)
	_, randErr := rand.Read(tokenBytes)
	require.NoError(t, randErr)
	staticToken := hex.EncodeToString(tokenBytes)

	// Create htpasswd file.
	// nik:letmein (APR1 hash).
	htpasswdFile := fmt.Sprintf("%s/htpasswd", tmpDir)
	htpasswdContent := "nik:$apr1$ytmDEY.X$LJt5T3fWtswK3KF5iINxT1"
	writeErr := os.WriteFile(htpasswdFile, []byte(htpasswdContent), 0644)
	require.NoError(t, writeErr)

	repoDir := fmt.Sprintf("%s/repo", tmpDir)
	mkdirErr := os.Mkdir(repoDir, 0755)
	require.NoError(t, mkdirErr)

	port, portErr := freeport.GetFreePort()
	require.NoError(t, portErr)

	// Create config with multi-auth: static-token,basic-htpasswd.
	configFile := fmt.Sprintf("%s/config.json", tmpDir)
	configContent := fmt.Sprintf(`{
  "address": "127.0.0.1",
  "port": %d,
  "serverRoot": "%s",
  "authTypePut": "static-token,basic-htpasswd",
  "authTypeGet": "static-token,basic-htpasswd",
  "authGets": true,
  "authOptsGet": {
    "staticToken": "%s",
    "idpFile": "%s"
  },
  "authOptsPut": {
    "staticToken": "%s",
    "idpFile": "%s"
  }
}`, port, repoDir, staticToken, htpasswdFile, staticToken, htpasswdFile)

	writeErr = os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, writeErr)

	server, serverErr := NewRepoServer(configFile)
	require.NoError(t, serverErr)

	go server.RunRepoServer()
	time.Sleep(time.Second)

	testHost := fmt.Sprintf("http://%s", net.JoinHostPort(server.Address, strconv.Itoa(server.Port)))
	client := &http.Client{}

	t.Run("PUT with static token", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/tokenfile", testHost)
		req, reqErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("token content")))
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+staticToken)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("PUT with basic auth", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/basicfile", testHost)
		req, reqErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("basic content")))
		require.NoError(t, reqErr)
		req.SetBasicAuth("nik", "letmein")

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("PUT without auth", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/noauthfile", testHost)
		req, reqErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("no auth")))
		require.NoError(t, reqErr)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("PUT with wrong token", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/wrongtoken", testHost)
		req, reqErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("wrong")))
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer wrong-token")

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("PUT with wrong basic auth", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/wrongbasic", testHost)
		req, reqErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("wrong")))
		require.NoError(t, reqErr)
		req.SetBasicAuth("nik", "wrongpassword")

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("GET with static token", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/tokenfile", testHost)
		req, reqErr := http.NewRequest(http.MethodGet, fileURL, nil)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+staticToken)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		defer resp.Body.Close()
		assert.Equal(t, "token content", string(body))
	})

	t.Run("GET with basic auth", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/basicfile", testHost)
		req, reqErr := http.NewRequest(http.MethodGet, fileURL, nil)
		require.NoError(t, reqErr)
		req.SetBasicAuth("nik", "letmein")

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, readErr := io.ReadAll(resp.Body)
		require.NoError(t, readErr)
		defer resp.Body.Close()
		assert.Equal(t, "basic content", string(body))
	})

	t.Run("GET without auth", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/tokenfile", testHost)
		req, reqErr := http.NewRequest(http.MethodGet, fileURL, nil)
		require.NoError(t, reqErr)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("DELETE with static token", func(t *testing.T) {
		// First PUT a file to delete.
		fileURL := fmt.Sprintf("%s/deleteme", testHost)
		putReq, putErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("delete this")))
		require.NoError(t, putErr)
		putReq.Header.Set("Authorization", "Bearer "+staticToken)

		putResp, putDoErr := client.Do(putReq)
		require.NoError(t, putDoErr)
		require.Equal(t, http.StatusCreated, putResp.StatusCode)

		// Now delete it.
		delReq, delErr := http.NewRequest(http.MethodDelete, fileURL, nil)
		require.NoError(t, delErr)
		delReq.Header.Set("Authorization", "Bearer "+staticToken)

		delResp, delDoErr := client.Do(delReq)
		require.NoError(t, delDoErr)
		assert.Equal(t, http.StatusNoContent, delResp.StatusCode)
	})

	t.Run("DELETE without auth", func(t *testing.T) {
		fileURL := fmt.Sprintf("%s/tokenfile", testHost)
		req, reqErr := http.NewRequest(http.MethodDelete, fileURL, nil)
		require.NoError(t, reqErr)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
