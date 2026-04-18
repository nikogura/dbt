package dbt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		setEnv       bool
		defaultValue string
		expected     string
	}{
		{
			name:         "returns env value when set",
			key:          "TEST_ENV_OR_DEFAULT_SET",
			envValue:     "custom-value",
			setEnv:       true,
			defaultValue: "default",
			expected:     "custom-value",
		},
		{
			name:         "returns default when env not set",
			key:          "TEST_ENV_OR_DEFAULT_UNSET",
			setEnv:       false,
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "returns default when env is empty",
			key:          "TEST_ENV_OR_DEFAULT_EMPTY",
			envValue:     "",
			setEnv:       true,
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(tc.key, tc.envValue)
			}

			result := envOrDefault(tc.key, tc.defaultValue)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEnvBool(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string
		setEnv   bool
		expected bool
	}{
		{
			name:     "true when set to true",
			key:      "TEST_ENV_BOOL_TRUE",
			envValue: "true",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "true when set to TRUE",
			key:      "TEST_ENV_BOOL_UPPER",
			envValue: "TRUE",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "true when set to True",
			key:      "TEST_ENV_BOOL_MIXED",
			envValue: "True",
			setEnv:   true,
			expected: true,
		},
		{
			name:     "false when set to false",
			key:      "TEST_ENV_BOOL_FALSE",
			envValue: "false",
			setEnv:   true,
			expected: false,
		},
		{
			name:     "false when set to other string",
			key:      "TEST_ENV_BOOL_OTHER",
			envValue: "yes",
			setEnv:   true,
			expected: false,
		},
		{
			name:     "false when not set",
			key:      "TEST_ENV_BOOL_UNSET",
			setEnv:   false,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(tc.key, tc.envValue)
			}

			result := envBool(tc.key)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEnvInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		envValue     string
		setEnv       bool
		defaultValue int
		expected     int
	}{
		{
			name:         "parses valid integer",
			key:          "TEST_ENV_INT_VALID",
			envValue:     "8080",
			setEnv:       true,
			defaultValue: 9999,
			expected:     8080,
		},
		{
			name:         "returns default for invalid integer",
			key:          "TEST_ENV_INT_INVALID",
			envValue:     "not-a-number",
			setEnv:       true,
			defaultValue: 9999,
			expected:     9999,
		},
		{
			name:         "returns default when not set",
			key:          "TEST_ENV_INT_UNSET",
			setEnv:       false,
			defaultValue: 9999,
			expected:     9999,
		},
		{
			name:         "returns default for empty string",
			key:          "TEST_ENV_INT_EMPTY",
			envValue:     "",
			setEnv:       true,
			defaultValue: 9999,
			expected:     9999,
		},
		{
			name:         "parses zero",
			key:          "TEST_ENV_INT_ZERO",
			envValue:     "0",
			setEnv:       true,
			defaultValue: 9999,
			expected:     0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(tc.key, tc.envValue)
			}

			result := envInt(tc.key, tc.defaultValue)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEnvSlice(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		envValue string
		setEnv   bool
		expected []string
	}{
		{
			name:     "splits comma-separated values",
			key:      "TEST_ENV_SLICE_CSV",
			envValue: "a,b,c",
			setEnv:   true,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "trims whitespace around values",
			key:      "TEST_ENV_SLICE_SPACES",
			envValue: " a , b , c ",
			setEnv:   true,
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "single value",
			key:      "TEST_ENV_SLICE_SINGLE",
			envValue: "only-one",
			setEnv:   true,
			expected: []string{"only-one"},
		},
		{
			name:     "nil when not set",
			key:      "TEST_ENV_SLICE_UNSET",
			setEnv:   false,
			expected: nil,
		},
		{
			name:     "nil when empty",
			key:      "TEST_ENV_SLICE_EMPTY",
			envValue: "",
			setEnv:   true,
			expected: nil,
		},
		{
			name:     "skips empty entries from trailing comma",
			key:      "TEST_ENV_SLICE_TRAIL",
			envValue: "a,b,",
			setEnv:   true,
			expected: []string{"a", "b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnv {
				t.Setenv(tc.key, tc.envValue)
			}

			result := envSlice(tc.key)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestOidcOptsFromEnv(t *testing.T) {
	t.Run("returns nil when issuer URL not set", func(t *testing.T) {
		// No env vars set — should return nil
		opts := oidcOptsFromEnv("TESTNIL")
		assert.Nil(t, opts)
	})

	t.Run("builds opts from env vars", func(t *testing.T) {
		t.Setenv("REPOSERVER_OIDC_ISSUER_URL_TESTFULL", "https://issuer.example.com")
		t.Setenv("REPOSERVER_OIDC_AUDIENCES_TESTFULL", "aud1,aud2")
		t.Setenv("REPOSERVER_OIDC_USERNAME_CLAIM_TESTFULL", "email")
		t.Setenv("REPOSERVER_OIDC_ALLOWED_GROUPS_TESTFULL", "group-a,group-b")
		t.Setenv("REPOSERVER_OIDC_SKIP_ISSUER_VERIFY_TESTFULL", "true")
		t.Setenv("REPOSERVER_OIDC_JWKS_CACHE_TESTFULL", "600")

		opts := oidcOptsFromEnv("TESTFULL")
		assert.NotNil(t, opts)
		assert.Equal(t, "https://issuer.example.com", opts.IssuerURL)
		assert.Equal(t, []string{"aud1", "aud2"}, opts.Audiences)
		assert.Equal(t, "email", opts.UsernameClaimKey)
		assert.Equal(t, []string{"group-a", "group-b"}, opts.AllowedGroups)
		assert.True(t, opts.SkipIssuerVerify)
		assert.Equal(t, 600, opts.JWKSCacheSeconds)
	})

	t.Run("issuer URL only produces minimal opts", func(t *testing.T) {
		t.Setenv("REPOSERVER_OIDC_ISSUER_URL_TESTMIN", "https://minimal.example.com")

		opts := oidcOptsFromEnv("TESTMIN")
		assert.NotNil(t, opts)
		assert.Equal(t, "https://minimal.example.com", opts.IssuerURL)
		assert.Nil(t, opts.Audiences)
		assert.Empty(t, opts.UsernameClaimKey)
		assert.Nil(t, opts.AllowedGroups)
		assert.False(t, opts.SkipIssuerVerify)
		assert.Equal(t, 0, opts.JWKSCacheSeconds)
	})
}

func TestNewRepoServerFromEnv(t *testing.T) {
	t.Run("defaults when no env vars set", func(t *testing.T) {
		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, "0.0.0.0", server.Address)
		assert.Equal(t, 9999, server.Port)
		assert.Empty(t, server.ServerRoot)
		assert.False(t, server.AuthGets)
		assert.Empty(t, server.AuthTypeGet)
		assert.Empty(t, server.AuthTypePut)
		assert.Nil(t, server.AuthOptsGet.OIDC)
		assert.Nil(t, server.AuthOptsPut.OIDC)
	})

	t.Run("basic fields from env", func(t *testing.T) {
		t.Setenv("REPOSERVER_ADDRESS", "10.0.0.1")
		t.Setenv("REPOSERVER_PORT", "8080")
		t.Setenv("REPOSERVER_ROOT", "/data/dbt")

		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.1", server.Address)
		assert.Equal(t, 8080, server.Port)
		assert.Equal(t, "/data/dbt", server.ServerRoot)
	})

	t.Run("auth fields from env", func(t *testing.T) {
		t.Setenv("REPOSERVER_AUTH_GETS", "true")
		t.Setenv("REPOSERVER_AUTH_TYPE_GET", "oidc")
		t.Setenv("REPOSERVER_AUTH_TYPE_PUT", "static-token,oidc")

		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.True(t, server.AuthGets)
		assert.Equal(t, "oidc", server.AuthTypeGet)
		assert.Equal(t, "static-token,oidc", server.AuthTypePut)
	})

	t.Run("static token and idp fields", func(t *testing.T) {
		t.Setenv("REPOSERVER_STATIC_TOKEN_PUT", "my-secret-token")
		t.Setenv("REPOSERVER_IDP_FILE_GET", "/etc/dbt/htpasswd-get")
		t.Setenv("REPOSERVER_IDP_FILE_PUT", "/etc/dbt/htpasswd-put")
		t.Setenv("REPOSERVER_IDP_FUNC_GET", "get-func")
		t.Setenv("REPOSERVER_IDP_FUNC_PUT", "put-func")

		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "my-secret-token", server.AuthOptsPut.StaticToken)
		assert.Equal(t, "/etc/dbt/htpasswd-get", server.AuthOptsGet.IdpFile)
		assert.Equal(t, "/etc/dbt/htpasswd-put", server.AuthOptsPut.IdpFile)
		assert.Equal(t, "get-func", server.AuthOptsGet.IdpFunc)
		assert.Equal(t, "put-func", server.AuthOptsPut.IdpFunc)
	})

	t.Run("OIDC config for GET only", func(t *testing.T) {
		t.Setenv("REPOSERVER_OIDC_ISSUER_URL_GET", "https://get-issuer.example.com")
		t.Setenv("REPOSERVER_OIDC_AUDIENCES_GET", "dbt-server")

		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.NotNil(t, server.AuthOptsGet.OIDC)
		assert.Equal(t, "https://get-issuer.example.com", server.AuthOptsGet.OIDC.IssuerURL)
		assert.Equal(t, []string{"dbt-server"}, server.AuthOptsGet.OIDC.Audiences)
		assert.Nil(t, server.AuthOptsPut.OIDC)
	})

	t.Run("OIDC config for PUT only", func(t *testing.T) {
		t.Setenv("REPOSERVER_OIDC_ISSUER_URL_PUT", "https://put-issuer.example.com")
		t.Setenv("REPOSERVER_OIDC_AUDIENCES_PUT", "dbt-server,dbt-admin")
		t.Setenv("REPOSERVER_OIDC_ALLOWED_GROUPS_PUT", "dbt-publishers")

		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.Nil(t, server.AuthOptsGet.OIDC)
		assert.NotNil(t, server.AuthOptsPut.OIDC)
		assert.Equal(t, "https://put-issuer.example.com", server.AuthOptsPut.OIDC.IssuerURL)
		assert.Equal(t, []string{"dbt-server", "dbt-admin"}, server.AuthOptsPut.OIDC.Audiences)
		assert.Equal(t, []string{"dbt-publishers"}, server.AuthOptsPut.OIDC.AllowedGroups)
	})

	t.Run("full configuration from env", func(t *testing.T) {
		t.Setenv("REPOSERVER_ADDRESS", "0.0.0.0")
		t.Setenv("REPOSERVER_PORT", "9999")
		t.Setenv("REPOSERVER_ROOT", "/var/dbt")
		t.Setenv("REPOSERVER_AUTH_GETS", "true")
		t.Setenv("REPOSERVER_AUTH_TYPE_GET", "oidc")
		t.Setenv("REPOSERVER_AUTH_TYPE_PUT", "static-token,oidc")
		t.Setenv("REPOSERVER_STATIC_TOKEN_PUT", "ci-token")
		t.Setenv("REPOSERVER_OIDC_ISSUER_URL_GET", "https://dex.example.com")
		t.Setenv("REPOSERVER_OIDC_AUDIENCES_GET", "dbt-server")
		t.Setenv("REPOSERVER_OIDC_USERNAME_CLAIM_GET", "email")
		t.Setenv("REPOSERVER_OIDC_ISSUER_URL_PUT", "https://dex.example.com")
		t.Setenv("REPOSERVER_OIDC_AUDIENCES_PUT", "dbt-server")
		t.Setenv("REPOSERVER_OIDC_ALLOWED_GROUPS_PUT", "dbt-publishers,admins")

		server, err := NewRepoServerFromEnv()
		require.NoError(t, err)
		assert.Equal(t, "0.0.0.0", server.Address)
		assert.Equal(t, 9999, server.Port)
		assert.Equal(t, "/var/dbt", server.ServerRoot)
		assert.True(t, server.AuthGets)
		assert.Equal(t, "oidc", server.AuthTypeGet)
		assert.Equal(t, "static-token,oidc", server.AuthTypePut)
		assert.Equal(t, "ci-token", server.AuthOptsPut.StaticToken)

		assert.NotNil(t, server.AuthOptsGet.OIDC)
		assert.Equal(t, "https://dex.example.com", server.AuthOptsGet.OIDC.IssuerURL)
		assert.Equal(t, []string{"dbt-server"}, server.AuthOptsGet.OIDC.Audiences)
		assert.Equal(t, "email", server.AuthOptsGet.OIDC.UsernameClaimKey)

		assert.NotNil(t, server.AuthOptsPut.OIDC)
		assert.Equal(t, "https://dex.example.com", server.AuthOptsPut.OIDC.IssuerURL)
		assert.Equal(t, []string{"dbt-server"}, server.AuthOptsPut.OIDC.Audiences)
		assert.Equal(t, []string{"dbt-publishers", "admins"}, server.AuthOptsPut.OIDC.AllowedGroups)
	})
}
