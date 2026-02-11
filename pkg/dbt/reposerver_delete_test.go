package dbt

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleDelete(t *testing.T) {
	deleteDir := t.TempDir()

	server := &DBTRepoServer{
		ServerRoot: deleteDir,
	}

	t.Run("delete file", func(t *testing.T) {
		testFile := filepath.Join(deleteDir, "testfile.txt")
		writeErr := os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, writeErr)

		statusCode, deleteErr := server.HandleDelete("/testfile.txt")
		require.NoError(t, deleteErr)
		assert.Equal(t, http.StatusNoContent, statusCode)

		_, statErr := os.Stat(testFile)
		assert.True(t, os.IsNotExist(statErr), "file should be deleted")
	})

	t.Run("delete directory", func(t *testing.T) {
		testDir := filepath.Join(deleteDir, "testdir")
		mkdirErr := os.MkdirAll(filepath.Join(testDir, "subdir"), 0755)
		require.NoError(t, mkdirErr)

		writeErr := os.WriteFile(filepath.Join(testDir, "subdir", "file.txt"), []byte("content"), 0644)
		require.NoError(t, writeErr)

		statusCode, deleteErr := server.HandleDelete("/testdir")
		require.NoError(t, deleteErr)
		assert.Equal(t, http.StatusNoContent, statusCode)

		_, statErr := os.Stat(testDir)
		assert.True(t, os.IsNotExist(statErr), "directory should be deleted")
	})
}

func TestHandleDeleteNotFound(t *testing.T) {
	notFoundDir := t.TempDir()

	server := &DBTRepoServer{
		ServerRoot: notFoundDir,
	}

	statusCode, deleteErr := server.HandleDelete("/nonexistent")
	require.Error(t, deleteErr)
	assert.Equal(t, http.StatusNotFound, statusCode)
}

func TestValidatePathWithinRoot(t *testing.T) {
	validateDir := t.TempDir()

	server := &DBTRepoServer{
		ServerRoot: validateDir,
	}

	tests := []struct {
		name        string
		requestPath string
		expectErr   bool
	}{
		{
			name:        "valid path",
			requestPath: "/tools/mytool/1.0.0",
			expectErr:   false,
		},
		{
			name:        "traversal with dot-dot",
			requestPath: "/../../../etc/passwd",
			expectErr:   true,
		},
		{
			name:        "absolute path within root",
			requestPath: "/tmp/evil",
			expectErr:   false, // /tmp/evil is joined with root, so it's within root
		},
		{
			name:        "nested traversal",
			requestPath: "/tools/../../..",
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvedPath, validateErr := server.ValidatePathWithinRoot(tt.requestPath)
			if tt.expectErr {
				require.Error(t, validateErr)
			} else {
				require.NoError(t, validateErr)
				assert.NotEmpty(t, resolvedPath)
			}
		})
	}
}

func TestDeleteHandlerStaticTokenAuth(t *testing.T) {
	authDir := t.TempDir()

	repoDir := filepath.Join(authDir, "repo")
	mkErr := os.Mkdir(repoDir, 0755)
	require.NoError(t, mkErr)

	// Generate a random token for testing
	tokenBytes := make([]byte, 32)
	_, randErr := rand.Read(tokenBytes)
	require.NoError(t, randErr)
	testToken := hex.EncodeToString(tokenBytes)

	deletePort, portErr := freeport.GetFreePort()
	require.NoError(t, portErr)

	server := &DBTRepoServer{
		Address:     "127.0.0.1",
		Port:        deletePort,
		ServerRoot:  repoDir,
		AuthTypePut: AUTH_STATIC_TOKEN,
		AuthOptsPut: AuthOpts{
			StaticToken: testToken,
		},
	}

	go server.RunRepoServer()

	time.Sleep(time.Second)

	deleteTestHost := fmt.Sprintf("http://%s", net.JoinHostPort(server.Address, strconv.Itoa(server.Port)))
	client := &http.Client{}

	// First PUT a file
	fileURL := fmt.Sprintf("%s/testfile", deleteTestHost)
	putReq, putErr := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte("test content")))
	require.NoError(t, putErr)
	putReq.Header.Set("Authorization", "Bearer "+testToken)

	putResp, putDoErr := client.Do(putReq)
	require.NoError(t, putDoErr)
	assert.Equal(t, http.StatusCreated, putResp.StatusCode)

	t.Run("unauthenticated delete", func(t *testing.T) {
		req, reqErr := http.NewRequest(http.MethodDelete, fileURL, nil)
		require.NoError(t, reqErr)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("authenticated delete", func(t *testing.T) {
		req, reqErr := http.NewRequest(http.MethodDelete, fileURL, nil)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+testToken)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		// Verify file is gone
		getReq, getErr := http.NewRequest(http.MethodGet, fileURL, nil)
		require.NoError(t, getErr)
		getReq.Header.Set("Authorization", "Bearer "+testToken)

		getResp, getDoErr := client.Do(getReq)
		require.NoError(t, getDoErr)
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
	})

	t.Run("delete not found", func(t *testing.T) {
		notFoundURL := fmt.Sprintf("%s/does-not-exist", deleteTestHost)
		req, reqErr := http.NewRequest(http.MethodDelete, notFoundURL, nil)
		require.NoError(t, reqErr)
		req.Header.Set("Authorization", "Bearer "+testToken)

		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestDeleteHandlerHtpasswdUnit(t *testing.T) {
	htpasswdDir := t.TempDir()

	// Create a test file to delete
	testFile := filepath.Join(htpasswdDir, "deleteme.txt")
	writeErr := os.WriteFile(testFile, []byte("delete this"), 0644)
	require.NoError(t, writeErr)

	server := &DBTRepoServer{
		ServerRoot: htpasswdDir,
	}

	// Use deleteAndRespond directly to test the delete logic
	w := httptest.NewRecorder()
	server.deleteAndRespond(w, "/deleteme.txt")

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	_, statErr := os.Stat(testFile)
	assert.True(t, os.IsNotExist(statErr))
}
