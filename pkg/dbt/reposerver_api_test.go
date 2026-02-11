package dbt

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIToolsHandler(t *testing.T) {
	apiToolsDir := t.TempDir()

	toolsDir := filepath.Join(apiToolsDir, "dbt-tools")
	mkErr := os.MkdirAll(toolsDir, 0755)
	require.NoError(t, mkErr)

	// Create some tool directories
	mkErr = os.Mkdir(filepath.Join(toolsDir, "tool-a"), 0755)
	require.NoError(t, mkErr)
	mkErr = os.Mkdir(filepath.Join(toolsDir, "tool-b"), 0755)
	require.NoError(t, mkErr)

	// Create a file (should be ignored)
	writeErr := os.WriteFile(filepath.Join(toolsDir, "readme.txt"), []byte("not a tool"), 0644)
	require.NoError(t, writeErr)

	server := &DBTRepoServer{
		ServerRoot: apiToolsDir,
	}

	req := httptest.NewRequest(http.MethodGet, "/-/api/tools", nil)
	w := httptest.NewRecorder()

	server.APIToolsHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	bodyBytes, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)

	var tools []ToolInfo
	unmarshalErr := json.Unmarshal(bodyBytes, &tools)
	require.NoError(t, unmarshalErr)

	assert.Len(t, tools, 2)

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "tool-a")
	assert.Contains(t, toolNames, "tool-b")
}

func TestAPIToolsHandlerEmpty(t *testing.T) {
	apiEmptyDir := t.TempDir()

	// Don't create dbt-tools dir - should return empty array
	server := &DBTRepoServer{
		ServerRoot: apiEmptyDir,
	}

	req := httptest.NewRequest(http.MethodGet, "/-/api/tools", nil)
	w := httptest.NewRecorder()

	server.APIToolsHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bodyBytes, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)
	assert.Equal(t, "[]", string(bodyBytes))
}

func TestAPIToolVersionsHandler(t *testing.T) {
	apiVersionsDir := t.TempDir()

	toolsDir := filepath.Join(apiVersionsDir, "dbt-tools", "mytool")
	mkErr := os.MkdirAll(toolsDir, 0755)
	require.NoError(t, mkErr)

	// Create version directories
	mkErr = os.Mkdir(filepath.Join(toolsDir, "1.0.0"), 0755)
	require.NoError(t, mkErr)
	mkErr = os.Mkdir(filepath.Join(toolsDir, "1.1.0"), 0755)
	require.NoError(t, mkErr)
	mkErr = os.Mkdir(filepath.Join(toolsDir, "2.0.0"), 0755)
	require.NoError(t, mkErr)

	// Create a non-semver directory (should be filtered out)
	mkErr = os.Mkdir(filepath.Join(toolsDir, "latest"), 0755)
	require.NoError(t, mkErr)

	server := &DBTRepoServer{
		ServerRoot: apiVersionsDir,
	}

	// Use gorilla/mux to parse the route variable
	r := mux.NewRouter()
	r.HandleFunc("/-/api/tools/{name}/versions", server.APIToolVersionsHandler).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/-/api/tools/mytool/versions", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	bodyBytes, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)

	var versions []VersionInfo
	unmarshalErr := json.Unmarshal(bodyBytes, &versions)
	require.NoError(t, unmarshalErr)

	assert.Len(t, versions, 3)

	versionNames := make([]string, len(versions))
	for i, v := range versions {
		versionNames[i] = v.Version
	}
	assert.Contains(t, versionNames, "1.0.0")
	assert.Contains(t, versionNames, "1.1.0")
	assert.Contains(t, versionNames, "2.0.0")
	assert.NotContains(t, versionNames, "latest")

	// Verify ModifiedAt is set
	for _, v := range versions {
		assert.False(t, v.ModifiedAt.IsZero(), "ModifiedAt should be set for version %s", v.Version)
	}
}

func TestAPIToolVersionsHandlerNotFound(t *testing.T) {
	apiNotFoundDir := t.TempDir()

	server := &DBTRepoServer{
		ServerRoot: apiNotFoundDir,
	}

	r := mux.NewRouter()
	r.HandleFunc("/-/api/tools/{name}/versions", server.APIToolVersionsHandler).Methods("GET")

	req := httptest.NewRequest(http.MethodGet, "/-/api/tools/nonexistent/versions", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}
