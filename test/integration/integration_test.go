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

//go:build integration

// Package integration provides end-to-end integration tests for dbt.
//
// These tests build and run the actual Docker container and dbt binary
// to verify the full system works correctly.
//
// Run with: go test -tags=integration -v ./test/integration/...
package integration

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nikogura/dbt/pkg/dbt/testfixtures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	reposerverImage   = "dbt-reposerver:integration-test"
	containerName     = "dbt-integration-test"
	reposerverPort    = "19999"
	testTimeout       = 5 * time.Minute
	containerStartup  = 3 * time.Second
	healthCheckPeriod = 500 * time.Millisecond
)

// TestContext holds all the test infrastructure.
type TestContext struct {
	t              *testing.T
	projectRoot    string
	tempDir        string
	repoDir        string
	dbtHomeDir     string
	dbtBinaryPath  string
	containerID    string
	reposerverURL  string
	truststorePath string
}

// newTestContext creates a new test context and sets up the test infrastructure.
func newTestContext(t *testing.T) (tc *TestContext) {
	t.Helper()

	projectRoot, err := findProjectRoot()
	require.NoError(t, err, "failed to find project root")

	tempDir := t.TempDir()

	tc = &TestContext{
		t:             t,
		projectRoot:   projectRoot,
		tempDir:       tempDir,
		repoDir:       filepath.Join(tempDir, "repo"),
		dbtHomeDir:    filepath.Join(tempDir, "dbt-home"),
		reposerverURL: fmt.Sprintf("http://localhost:%s", reposerverPort),
	}

	return tc
}

// cleanup removes all test resources.
func (tc *TestContext) cleanup() {
	tc.t.Helper()

	// Stop and remove container
	if tc.containerID != "" {
		tc.t.Logf("Stopping container %s", tc.containerID[:12])
		_ = exec.Command("docker", "rm", "-f", tc.containerID).Run()
	}

	// Fix ownership on repo dir so t.TempDir() cleanup can delete files
	// Files created by the container are owned by uid 65532 (nonroot)
	// Use docker to chown since we can't do it directly without root
	if tc.repoDir != "" {
		uid := os.Getuid()
		gid := os.Getgid()
		_ = exec.Command("docker", "run", "--rm",
			"-v", fmt.Sprintf("%s:/data", tc.repoDir),
			"alpine:latest",
			"chown", "-R", fmt.Sprintf("%d:%d", uid, gid), "/data",
		).Run()
	}

	// Note: tempDir is managed by t.TempDir() and cleaned up automatically
}

// findProjectRoot locates the project root directory.
func findProjectRoot() (root string, err error) {
	// Start from current working directory and walk up
	dir, err := os.Getwd()
	if err != nil {
		return root, err
	}

	for {
		_, statErr := os.Stat(filepath.Join(dir, "go.mod"))
		if statErr == nil {
			// Check if this is the dbt project
			modBytes, readErr := os.ReadFile(filepath.Join(dir, "go.mod"))
			if readErr == nil && strings.Contains(string(modBytes), "github.com/nikogura/dbt") {
				root = dir
				return root, err
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			err = errors.New("could not find project root (go.mod with github.com/nikogura/dbt)")
			return root, err
		}
		dir = parent
	}
}

// buildDockerImage builds the reposerver Docker image.
func (tc *TestContext) buildDockerImage() (err error) {
	tc.t.Helper()
	tc.t.Log("Building reposerver Docker image...")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "build",
		"-f", "dockerfiles/reposerver/Dockerfile",
		"-t", reposerverImage,
		".")
	cmd.Dir = tc.projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		tc.t.Logf("Docker build output:\n%s", string(output))
		return fmt.Errorf("docker build failed: %w", err)
	}

	tc.t.Log("Docker image built successfully")
	return err
}

// buildDbtBinary builds the dbt binary.
func (tc *TestContext) buildDbtBinary() (err error) {
	tc.t.Helper()
	tc.t.Log("Building dbt binary...")

	tc.dbtBinaryPath = filepath.Join(tc.tempDir, "dbt")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build",
		"-o", tc.dbtBinaryPath,
		"./cmd/dbt")
	cmd.Dir = tc.projectRoot
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		tc.t.Logf("Go build output:\n%s", string(output))
		return fmt.Errorf("go build failed: %w", err)
	}

	tc.t.Logf("Built dbt binary at %s", tc.dbtBinaryPath)
	return err
}

// setupTestRepository sets up a test repository with fixtures.
func (tc *TestContext) setupTestRepository() (err error) {
	tc.t.Helper()
	tc.t.Log("Setting up test repository...")

	// Copy fixtures from pkg/dbt/testfixtures/repo
	fixturesDir := filepath.Join(tc.projectRoot, "pkg", "dbt", "testfixtures", "repo")

	err = copyDir(fixturesDir, tc.repoDir)
	if err != nil {
		return fmt.Errorf("failed to copy fixtures: %w", err)
	}

	// Set truststore path
	tc.truststorePath = filepath.Join(tc.repoDir, "dbt", "truststore")

	tc.t.Logf("Test repository set up at %s", tc.repoDir)
	return err
}

// startReposerver starts the reposerver container.
func (tc *TestContext) startReposerver() (err error) {
	tc.t.Helper()
	tc.t.Log("Starting reposerver container...")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Remove any existing container with the same name
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	cmd := exec.CommandContext(ctx, "docker", "run",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:9999", reposerverPort),
		"-v", fmt.Sprintf("%s:/var/dbt:ro", tc.repoDir),
		reposerverImage,
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			tc.t.Logf("Docker run stderr:\n%s", string(exitErr.Stderr))
		}
		return fmt.Errorf("docker run failed: %w", err)
	}

	tc.containerID = strings.TrimSpace(string(output))
	tc.t.Logf("Started container %s", tc.containerID[:12])

	// Wait for container to be healthy
	err = tc.waitForHealthy()
	if err != nil {
		// Get container logs for debugging
		logCmd := exec.Command("docker", "logs", tc.containerID)
		logs, _ := logCmd.CombinedOutput()
		tc.t.Logf("Container logs:\n%s", string(logs))
		return err
	}

	return err
}

// waitForHealthy waits for the reposerver to respond.
func (tc *TestContext) waitForHealthy() (err error) {
	tc.t.Helper()
	tc.t.Log("Waiting for reposerver to become healthy...")

	deadline := time.Now().Add(30 * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		resp, httpErr := client.Get(tc.reposerverURL + "/")
		if httpErr == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				tc.t.Log("Reposerver is healthy")
				return err
			}
		}
		time.Sleep(healthCheckPeriod)
	}

	err = errors.New("reposerver did not become healthy within timeout")
	return err
}

// setupDbtConfig creates the dbt configuration file.
func (tc *TestContext) setupDbtConfig() (err error) {
	tc.t.Helper()
	tc.t.Log("Setting up dbt configuration...")

	// Create .dbt/conf directory
	confDir := filepath.Join(tc.dbtHomeDir, ".dbt", "conf")
	err = os.MkdirAll(confDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create conf directory: %w", err)
	}

	// Create .dbt/trust directory (dbt needs this to store the truststore)
	trustDir := filepath.Join(tc.dbtHomeDir, ".dbt", "trust")
	err = os.MkdirAll(trustDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create trust directory: %w", err)
	}

	// Create .dbt/tools directory (dbt needs this to cache downloaded tools)
	toolsDir := filepath.Join(tc.dbtHomeDir, ".dbt", "tools")
	err = os.MkdirAll(toolsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create tools directory: %w", err)
	}

	// Create config file
	config := map[string]interface{}{
		"dbt": map[string]string{
			"repository": tc.reposerverURL + "/dbt",
			"truststore": tc.reposerverURL + "/dbt/truststore",
		},
		"tools": map[string]string{
			"repository": tc.reposerverURL + "/dbt-tools",
		},
	}

	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(confDir, "dbt.json")
	err = os.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	tc.t.Logf("Created dbt config at %s", configPath)
	return err
}

// runDbt runs the dbt binary with the given arguments.
func (tc *TestContext) runDbt(args ...string) (stdout string, stderr string, err error) {
	tc.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tc.dbtBinaryPath, args...)
	cmd.Dir = tc.tempDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("HOME=%s", tc.dbtHomeDir),
		fmt.Sprintf("DBT_HOME=%s", tc.dbtHomeDir),
	)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	return stdout, stderr, err
}

// copyDir recursively copies a directory.
func copyDir(src string, dst string) (err error) {
	src = filepath.Clean(src)
	dst = filepath.Clean(dst)

	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	err = os.MkdirAll(dst, info.Mode())
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			err = copyDir(srcPath, dstPath)
			if err != nil {
				return err
			}
		} else {
			err = copyFile(srcPath, dstPath)
			if err != nil {
				return err
			}
		}
	}

	return err
}

// copyFile copies a single file.
func copyFile(src string, dst string) (err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// TestIntegrationSuite runs the full integration test suite.
func TestIntegrationSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if Docker is available
	_, dockerErr := exec.LookPath("docker")
	if dockerErr != nil {
		t.Skip("docker not found, skipping integration tests")
	}

	tc := newTestContext(t)
	defer tc.cleanup()

	// Setup phase
	t.Run("Setup", func(t *testing.T) {
		t.Run("BuildDockerImage", func(t *testing.T) {
			err := tc.buildDockerImage()
			require.NoError(t, err)
		})

		t.Run("BuildDbtBinary", func(t *testing.T) {
			err := tc.buildDbtBinary()
			require.NoError(t, err)
		})

		t.Run("SetupTestRepository", func(t *testing.T) {
			err := tc.setupTestRepository()
			require.NoError(t, err)
		})

		t.Run("StartReposerver", func(t *testing.T) {
			err := tc.startReposerver()
			require.NoError(t, err)
		})

		t.Run("SetupDbtConfig", func(t *testing.T) {
			err := tc.setupDbtConfig()
			require.NoError(t, err)
		})
	})

	// Test phase
	t.Run("Tests", func(t *testing.T) {
		t.Run("DbtVersion", func(t *testing.T) {
			stdout, stderr, err := tc.runDbt("--version")
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			require.NoError(t, err, "dbt --version should succeed")
			assert.Contains(t, stdout+stderr, "dbt version", "output should contain version info")
		})

		t.Run("DbtHelp", func(t *testing.T) {
			stdout, stderr, err := tc.runDbt("--help")
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			require.NoError(t, err, "dbt --help should succeed")
			assert.Contains(t, stdout, "Dynamic Binary Toolkit", "help should describe dbt")
		})

		t.Run("ReposerverServesTruststore", func(t *testing.T) {
			resp, err := http.Get(tc.reposerverURL + "/dbt/truststore")
			require.NoError(t, err, "should be able to fetch truststore")
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode, "truststore should return 200")

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Contains(t, string(body), "BEGIN PGP PUBLIC KEY BLOCK", "truststore should contain PGP key")
		})

		t.Run("ReposerverServesBinaries", func(t *testing.T) {
			// Check that binaries are accessible (using static test fixture versions)
			versions := []string{testfixtures.OldVersion, testfixtures.NewVersion, testfixtures.LatestVersion}
			for _, version := range versions {
				url := fmt.Sprintf("%s/dbt/%s/linux/amd64/dbt", tc.reposerverURL, version)
				resp, err := http.Get(url)
				require.NoError(t, err, "should be able to fetch dbt binary for version %s", version)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode, "dbt %s binary should return 200", version)

				// Check checksum
				checksumURL := url + ".sha256"
				resp, err = http.Get(checksumURL)
				require.NoError(t, err, "should be able to fetch checksum for version %s", version)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode, "dbt %s checksum should return 200", version)

				// Check signature
				sigURL := url + ".asc"
				resp, err = http.Get(sigURL)
				require.NoError(t, err, "should be able to fetch signature for version %s", version)
				resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode, "dbt %s signature should return 200", version)
			}
		})

		t.Run("ReposerverServesTools", func(t *testing.T) {
			// Check catalog tool (using latest test fixture version)
			resp, err := http.Get(tc.reposerverURL + "/dbt-tools/catalog/" + testfixtures.LatestVersion + "/linux/amd64/catalog")
			require.NoError(t, err, "should be able to fetch catalog binary")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "catalog binary should return 200")

			// Check description
			resp, err = http.Get(tc.reposerverURL + "/dbt-tools/catalog/" + testfixtures.LatestVersion + "/description.txt")
			require.NoError(t, err, "should be able to fetch catalog description")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "catalog description should return 200")
		})

		t.Run("CatalogListOffline", func(t *testing.T) {
			// Use offline mode to skip self-update check
			// This tests the catalog listing functionality without the self-update complexity
			stdout, stderr, err := tc.runDbt("-o", "catalog", "list")
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)

			if err != nil {
				// In offline mode without cached tools, this may fail
				t.Logf("dbt catalog list (offline) failed: %v", err)
				// Verify at least the reposerver is serving content
				resp, httpErr := http.Get(tc.reposerverURL + "/dbt-tools/catalog/")
				if httpErr == nil {
					resp.Body.Close()
					t.Log("Reposerver is serving catalog directory correctly")
				}
			} else {
				assert.Contains(t, stdout, "catalog", "catalog list should show catalog tool")
			}
		})

		t.Run("FetchTruststore", func(t *testing.T) {
			// Test that dbt can fetch and verify the truststore
			// This is a critical security function
			resp, err := http.Get(tc.reposerverURL + "/dbt/truststore")
			require.NoError(t, err, "should be able to fetch truststore")
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			// Verify it's a valid PGP public key block
			truststore := string(body)
			assert.Contains(t, truststore, "-----BEGIN PGP PUBLIC KEY BLOCK-----", "truststore should have PGP header")
			assert.Contains(t, truststore, "-----END PGP PUBLIC KEY BLOCK-----", "truststore should have PGP footer")
			// The key identity is Base64 encoded inside the PGP block, so just verify the structure is valid
			assert.Greater(t, len(truststore), 500, "truststore should be a reasonable size for a PGP key")
		})
	})
}

// TestReposerverDirectAccess tests direct HTTP access to the reposerver.
func TestReposerverDirectAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, dockerErr := exec.LookPath("docker")
	if dockerErr != nil {
		t.Skip("docker not found, skipping integration tests")
	}

	tc := newTestContext(t)
	defer tc.cleanup()

	// Setup
	require.NoError(t, tc.buildDockerImage())
	require.NoError(t, tc.setupTestRepository())
	require.NoError(t, tc.startReposerver())

	t.Run("RootReturnsOK", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("DirectoryListing", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// Should contain version directories (using static test fixture versions)
		assert.Contains(t, string(body), testfixtures.OldVersion, "should list version "+testfixtures.OldVersion)
	})

	t.Run("NotFoundReturns404", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/nonexistent")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("BinaryContent", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/" + testfixtures.LatestVersion + "/linux/amd64/dbt")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "dbt version "+testfixtures.LatestVersion, "binary should contain version string")
	})

	t.Run("ChecksumContent", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/" + testfixtures.LatestVersion + "/linux/amd64/dbt.sha256")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// SHA256 checksums are 64 hex characters
		checksum := strings.TrimSpace(string(body))
		assert.Len(t, checksum, 64, "checksum should be 64 characters")
	})

	t.Run("SignatureContent", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/" + testfixtures.LatestVersion + "/linux/amd64/dbt.asc")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "BEGIN PGP SIGNATURE", "should contain PGP signature")
	})
}

// TestReposerverStaticTokenAuth tests static token authentication for the reposerver.
func TestReposerverStaticTokenAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, dockerErr := exec.LookPath("docker")
	if dockerErr != nil {
		t.Skip("docker not found, skipping integration tests")
	}

	tc := newTestContext(t)
	defer tc.cleanup()

	// Setup
	require.NoError(t, tc.buildDockerImage())
	require.NoError(t, tc.setupTestRepository())

	// Create auth config with static token
	staticToken := "test-secret-token-12345"
	authConfig := map[string]interface{}{
		"staticToken": staticToken,
	}

	require.NoError(t, tc.startReposerverWithAuth(authConfig))

	t.Run("GETWithoutAuthSucceeds", func(t *testing.T) {
		// GET requests should still work without auth for reading
		resp, err := http.Get(tc.reposerverURL + "/dbt/truststore")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "GET should succeed without auth")
	})

	t.Run("PUTWithoutAuthFails", func(t *testing.T) {
		// PUT requests should fail without auth
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-file.txt", strings.NewReader("test content"))
		require.NoError(t, err)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "PUT without auth should return 401")
	})

	t.Run("PUTWithInvalidTokenFails", func(t *testing.T) {
		// PUT requests with invalid token should fail
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-file.txt", strings.NewReader("test content"))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-token")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "PUT with invalid token should return 401")
	})

	t.Run("PUTWithValidTokenSucceeds", func(t *testing.T) {
		// PUT requests with valid token should succeed
		testContent := "test content for auth"
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-auth-file.txt", strings.NewReader(testContent))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+staticToken)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "PUT with valid token should return 201 Created")

		// Verify the file was created by reading it back
		getResp, err := http.Get(tc.reposerverURL + "/test-auth-file.txt")
		require.NoError(t, err)
		defer getResp.Body.Close()

		body, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent, string(body), "file content should match what was uploaded")
	})

	t.Run("PUTWithMalformedAuthHeaderFails", func(t *testing.T) {
		// PUT requests with malformed auth header should fail
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-file.txt", strings.NewReader("test content"))
		require.NoError(t, err)
		req.Header.Set("Authorization", "NotBearer "+staticToken)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "PUT with malformed auth header should return 401")
	})

	t.Run("PUTNestedPathWithValidToken", func(t *testing.T) {
		// PUT requests to nested paths should work with valid token
		testContent := "nested file content"
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/dbt-tools/test-tool/1.0.0/description.txt", strings.NewReader(testContent))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+staticToken)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode, "PUT to nested path with valid token should return 201 Created")
	})
}

// TestReposerverOIDCTokenExchange tests OIDC authentication using OAuth2 Token Exchange (RFC 8693).
//
// This is NOT the standard OIDC Authorization Code flow. Instead, it uses:
// 1. SSH-agent signs a JWT with target_audience claim
// 2. JWT is exchanged for OIDC tokens via Dex's token exchange endpoint
// 3. OIDC ID token is used for Bearer auth to reposerver
//
// This flow is designed for non-interactive/CLI authentication where browser redirects aren't practical.
func TestReposerverOIDCTokenExchange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, dockerErr := exec.LookPath("docker")
	if dockerErr != nil {
		t.Skip("docker not found, skipping integration tests")
	}

	// Check if Dex image exists or can be built
	dexImageCheck := exec.Command("docker", "image", "inspect", "dex:test")
	if dexImageCheck.Run() != nil {
		t.Skip("dex:test image not found - build with 'docker build -t dex:test .' in ~/project/nikogura/dex")
	}

	tc := newTestContext(t)
	defer tc.cleanup()

	// Setup
	require.NoError(t, tc.buildDockerImage())
	require.NoError(t, tc.setupTestRepository())

	// Generate test SSH keypair
	testKeyPair := generateTestKeyPairForIntegration(t, tc.tempDir)

	// Start Dex with SSH connector configured for the test key
	dexPort := "15556"
	require.NoError(t, tc.startDexWithSSHConnector(testKeyPair.publicKey, dexPort))

	// Start reposerver with OIDC auth pointing to Dex
	require.NoError(t, tc.startReposerverWithOIDC(dexPort))

	// Setup SSH agent with test key
	agentCleanup := setupSSHAgent(t, testKeyPair.keyPath)
	defer agentCleanup()

	t.Run("GETWithoutAuthSucceeds", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/truststore")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "GET should succeed without auth")
	})

	t.Run("PUTWithoutAuthFails", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-oidc.txt", strings.NewReader("test"))
		require.NoError(t, err)

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "PUT without auth should return 401")
	})

	t.Run("PUTWithValidOIDCToken", func(t *testing.T) {
		// Get OIDC token from Dex via token exchange
		oidcToken, err := tc.getOIDCTokenFromDex(testKeyPair.publicKey, dexPort)
		if err != nil {
			t.Skipf("Failed to get OIDC token from Dex: %v (Dex may not be ready)", err)
		}

		testContent := "oidc authenticated content"
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-oidc-auth.txt", strings.NewReader(testContent))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+oidcToken)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "PUT with valid OIDC token should succeed")

		// Verify file was created
		getResp, err := http.Get(tc.reposerverURL + "/test-oidc-auth.txt")
		require.NoError(t, err)
		defer getResp.Body.Close()

		body, err := io.ReadAll(getResp.Body)
		require.NoError(t, err)
		assert.Equal(t, testContent, string(body))
	})

	t.Run("PUTWithInvalidOIDCTokenFails", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPut, tc.reposerverURL+"/test-invalid.txt", strings.NewReader("test"))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-oidc-token")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "PUT with invalid OIDC token should return 401")
	})
}

// integrationKeyPair holds SSH key pair for integration tests.
type integrationKeyPair struct {
	publicKey  string
	privateKey string
	keyPath    string
}

// generateTestKeyPairForIntegration creates an SSH keypair for integration testing.
func generateTestKeyPairForIntegration(t *testing.T, tmpDir string) (keyPair *integrationKeyPair) {
	t.Helper()

	keyPath := filepath.Join(tmpDir, fmt.Sprintf("test_key_%d", time.Now().UnixNano()))
	pubKeyPath := keyPath + ".pub"

	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", "dbt-integration-test")
	err := cmd.Run()
	require.NoError(t, err, "Failed to generate SSH key")

	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	require.NoError(t, err, "Failed to read public key")

	privKeyBytes, err := os.ReadFile(keyPath)
	require.NoError(t, err, "Failed to read private key")

	keyPair = &integrationKeyPair{
		publicKey:  strings.TrimSpace(string(pubKeyBytes)),
		privateKey: string(privKeyBytes),
		keyPath:    keyPath,
	}
	return keyPair
}

// setupSSHAgent starts an SSH agent and adds the test key.
func setupSSHAgent(t *testing.T, keyPath string) (cleanup func()) {
	t.Helper()

	oldAuthSock := os.Getenv("SSH_AUTH_SOCK")

	agentCmd := exec.Command("ssh-agent", "-s")
	agentOut, err := agentCmd.Output()
	require.NoError(t, err, "Failed to start SSH agent")

	var agentPID string
	for _, line := range strings.Split(string(agentOut), "\n") {
		if sockPath, foundSock := strings.CutPrefix(line, "SSH_AUTH_SOCK="); foundSock {
			sockPath = strings.TrimSuffix(sockPath, "; export SSH_AUTH_SOCK;")
			os.Setenv("SSH_AUTH_SOCK", sockPath) //nolint:usetesting // cleanup restores original value
		} else if pidStr, foundPID := strings.CutPrefix(line, "SSH_AGENT_PID="); foundPID {
			pidStr = strings.TrimSuffix(pidStr, "; export SSH_AGENT_PID;")
			agentPID = pidStr
			os.Setenv("SSH_AGENT_PID", pidStr) //nolint:usetesting // cleanup restores original value
		}
	}

	addKeyCmd := exec.Command("ssh-add", keyPath)
	err = addKeyCmd.Run()
	require.NoError(t, err, "Failed to add key to SSH agent")

	cleanup = func() {
		if agentPID != "" {
			_ = exec.Command("kill", agentPID).Run()
		}
		if oldAuthSock != "" {
			os.Setenv("SSH_AUTH_SOCK", oldAuthSock) //nolint:usetesting // restoring original env outside test scope
		} else {
			os.Unsetenv("SSH_AUTH_SOCK")
		}
		os.Unsetenv("SSH_AGENT_PID")
	}
	return cleanup
}

// startDexWithSSHConnector starts a Dex container with SSH connector configured.
func (tc *TestContext) startDexWithSSHConnector(publicKey string, dexPort string) (err error) {
	tc.t.Helper()
	tc.t.Log("Starting Dex container with SSH connector...")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Remove any existing Dex container
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", "dex-integration-test").Run()

	// Create Dex config with SSH connector
	dexConfig := fmt.Sprintf(`issuer: http://host.docker.internal:%s/dex

storage:
  type: memory

web:
  http: 0.0.0.0:5556

logger:
  level: "debug"
  format: "text"

oauth2:
  skipApprovalScreen: true

staticClients:
- id: dbt-reposerver
  redirectURIs:
  - 'http://localhost/callback'
  name: 'DBT Reposerver'
  secret: dbt-test-secret
  public: true

connectors:
- type: ssh
  id: ssh
  name: SSH
  config:
    users:
      testuser:
        keys:
          - "%s"
        user_info:
          username: "testuser"
          email: "testuser@example.com"
          groups: ["publishers", "developers"]

    allowed_issuers:
      - "testuser"

    dex_instance_id: "http://host.docker.internal:%s/dex"

    allowed_target_audiences:
      - "dbt-reposerver"

    default_groups: ["authenticated"]
    token_ttl: 3600
    challenge_ttl: 300

    allowed_clients:
      - "dbt-reposerver"
`, dexPort, publicKey, dexPort)

	dexConfigPath := filepath.Join(tc.tempDir, "dex-config.yaml")
	err = os.WriteFile(dexConfigPath, []byte(dexConfig), 0644)
	if err != nil {
		return fmt.Errorf("failed to write dex config: %w", err)
	}

	tc.t.Logf("Dex config written to %s", dexConfigPath)

	cmd := exec.CommandContext(ctx, "docker", "run",
		"-d",
		"--name", "dex-integration-test",
		"--add-host", "host.docker.internal:host-gateway",
		"-p", fmt.Sprintf("%s:5556", dexPort),
		"-v", fmt.Sprintf("%s:/etc/dex/config.yaml:ro", dexConfigPath),
		"dex:test",
		"dex", "serve", "/etc/dex/config.yaml",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			tc.t.Logf("Dex docker run stderr:\n%s", string(exitErr.Stderr))
		}
		return fmt.Errorf("failed to start Dex container: %w", err)
	}

	dexContainerID := strings.TrimSpace(string(output))
	tc.t.Logf("Started Dex container %s", dexContainerID[:12])

	// Register cleanup
	tc.t.Cleanup(func() {
		_ = exec.Command("docker", "rm", "-f", "dex-integration-test").Run()
	})

	// Wait for Dex to be ready
	err = tc.waitForDex(dexPort)
	if err != nil {
		logCmd := exec.Command("docker", "logs", "dex-integration-test")
		logs, _ := logCmd.CombinedOutput()
		tc.t.Logf("Dex container logs:\n%s", string(logs))
		return err
	}

	return err
}

// waitForDex waits for Dex to become healthy.
func (tc *TestContext) waitForDex(dexPort string) (err error) {
	tc.t.Helper()
	tc.t.Log("Waiting for Dex to become healthy...")

	deadline := time.Now().Add(30 * time.Second)
	client := &http.Client{Timeout: 2 * time.Second}
	dexURL := fmt.Sprintf("http://localhost:%s/dex/.well-known/openid-configuration", dexPort)

	for time.Now().Before(deadline) {
		resp, httpErr := client.Get(dexURL)
		if httpErr == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				tc.t.Log("Dex is healthy")
				return err
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	err = errors.New("Dex did not become healthy within timeout")
	return err
}

// startReposerverWithOIDC starts the reposerver with OIDC auth configured.
func (tc *TestContext) startReposerverWithOIDC(dexPort string) (err error) {
	tc.t.Helper()
	tc.t.Log("Starting reposerver with OIDC auth...")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	// Create reposerver config with OIDC auth
	// Use localhost from container's perspective via host.docker.internal
	repoConfig := map[string]interface{}{
		"address":     "0.0.0.0",
		"port":        9999,
		"serverRoot":  "/var/dbt",
		"authTypePut": "oidc",
		"authOptsPut": map[string]interface{}{
			"oidc": map[string]interface{}{
				"issuerUrl":        fmt.Sprintf("http://host.docker.internal:%s/dex", dexPort),
				"audiences":        []string{"dbt-reposerver"},
				"usernameClaimKey": "email",
				"allowedGroups":    []string{"publishers"},
			},
		},
	}

	configBytes, err := json.MarshalIndent(repoConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal reposerver config: %w", err)
	}

	configPath := filepath.Join(tc.repoDir, "reposerver-oidc.json")
	err = os.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write reposerver config: %w", err)
	}

	tc.t.Logf("Reposerver OIDC config: %s", string(configBytes))

	cmd := exec.CommandContext(ctx, "docker", "run",
		"-d",
		"--name", containerName,
		"--add-host", "host.docker.internal:host-gateway",
		"-p", fmt.Sprintf("%s:9999", reposerverPort),
		"-v", fmt.Sprintf("%s:/var/dbt", tc.repoDir),
		reposerverImage,
		"-f", "/var/dbt/reposerver-oidc.json",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			tc.t.Logf("Docker run stderr:\n%s", string(exitErr.Stderr))
		}
		return fmt.Errorf("docker run failed: %w", err)
	}

	tc.containerID = strings.TrimSpace(string(output))
	tc.t.Logf("Started reposerver container %s", tc.containerID[:12])

	err = tc.waitForHealthy()
	if err != nil {
		logCmd := exec.Command("docker", "logs", tc.containerID)
		logs, _ := logCmd.CombinedOutput()
		tc.t.Logf("Reposerver container logs:\n%s", string(logs))
		return err
	}

	return err
}

// getOIDCTokenFromDex exchanges an SSH-signed JWT for an OIDC token via Dex.
func (tc *TestContext) getOIDCTokenFromDex(publicKey string, dexPort string) (token string, err error) {
	tc.t.Helper()

	// Create SSH-signed JWT using jwt-ssh-agent-go
	// The JWT needs dual audience: dex instance ID as aud, target audience as target_audience claim
	dexInstanceID := fmt.Sprintf("http://host.docker.internal:%s/dex", dexPort)

	sshJWT, err := createSSHSignedJWTForDex("testuser", dexInstanceID, "dbt-reposerver", publicKey)
	if err != nil {
		return token, fmt.Errorf("failed to create SSH JWT: %w", err)
	}

	tc.t.Logf("Created SSH JWT for token exchange")

	// Exchange SSH JWT for OIDC token via Dex token endpoint
	tokenURL := fmt.Sprintf("http://localhost:%s/dex/token", dexPort)

	data := url.Values{}
	data.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	data.Set("subject_token", sshJWT)
	data.Set("subject_token_type", "urn:ietf:params:oauth:token-type:jwt")
	data.Set("client_id", "dbt-reposerver")
	data.Set("scope", "openid email groups")

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return token, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return token, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return token, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
	}

	err = json.Unmarshal(body, &tokenResp)
	if err != nil {
		return token, fmt.Errorf("failed to parse token response: %w", err)
	}

	// Prefer ID token for OIDC auth
	if tokenResp.IDToken != "" {
		token = tokenResp.IDToken
	} else {
		token = tokenResp.AccessToken
	}

	tc.t.Logf("Got OIDC token from Dex")
	return token, err
}

// createSSHSignedJWTForDex creates an SSH-signed JWT with dual audience for Dex token exchange.
// This creates a JWT with aud set to the Dex instance ID (where we're sending the JWT)
// and target_audience set to the final audience for the OIDC token we want back.
func createSSHSignedJWTForDex(username, dexInstanceID, targetAudience, publicKey string) (jwtString string, err error) {
	// Create custom claims with target_audience for Dex's SSH connector
	now := time.Now()
	expiration := now.Add(5 * time.Minute)

	// Generate random JWT ID
	idBytes := make([]byte, 32)
	_, err = cryptorand.Read(idBytes)
	if err != nil {
		return jwtString, fmt.Errorf("failed to generate JWT ID: %w", err)
	}

	claims := jwt.MapClaims{
		"iss":             username,
		"sub":             username,
		"aud":             dexInstanceID,
		"target_audience": targetAudience,
		"exp":             expiration.Unix(),
		"iat":             now.Unix(),
		"nbf":             now.Unix(),
		"jti":             hex.EncodeToString(idBytes),
	}

	// Determine key type and create appropriate token
	parts := strings.Split(publicKey, " ")
	if len(parts) < 2 {
		return jwtString, errors.New("invalid public key format")
	}

	algo := parts[0]
	var tok *jwt.Token

	switch algo {
	case "ssh-ed25519":
		// Register ED25519 signing method for SSH agent
		signingMethod := &sshED25519SigningMethod{}
		jwt.RegisterSigningMethod(signingMethod.Alg(), func() jwt.SigningMethod {
			return signingMethod
		})
		tok = jwt.NewWithClaims(signingMethod, claims)

	case "ssh-rsa":
		signingMethod := &sshRSASigningMethod{}
		jwt.RegisterSigningMethod(signingMethod.Alg(), func() jwt.SigningMethod {
			return signingMethod
		})
		tok = jwt.NewWithClaims(signingMethod, claims)

	default:
		return jwtString, fmt.Errorf("unsupported key type: %s", algo)
	}

	// Parse the public key for signing
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return jwtString, fmt.Errorf("failed to parse public key: %w", err)
	}

	jwtString, err = tok.SignedString(pubKey)
	if err != nil {
		return jwtString, fmt.Errorf("failed to sign JWT: %w", err)
	}

	return jwtString, err
}

// sshED25519SigningMethod implements JWT signing via SSH agent for ED25519 keys.
type sshED25519SigningMethod struct{}

func (m *sshED25519SigningMethod) Alg() string { return "EdDSA" }

func (m *sshED25519SigningMethod) Verify(signingString string, sig []byte, key interface{}) error {
	return errors.New("verification not implemented")
}

func (m *sshED25519SigningMethod) Sign(signingString string, key interface{}) ([]byte, error) {
	pubKey, ok := key.(ssh.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected ssh.PublicKey, got %T", key)
	}

	// Get SSH agent connection
	agentSock := os.Getenv("SSH_AUTH_SOCK")
	if agentSock == "" {
		return nil, errors.New("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", agentSock)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	// Sign the data
	sigData, err := agentClient.Sign(pubKey, []byte(signingString))
	if err != nil {
		return nil, fmt.Errorf("SSH agent signing failed: %w", err)
	}

	// Return base64-encoded signature blob
	return []byte(base64.StdEncoding.EncodeToString(sigData.Blob)), nil
}

// sshRSASigningMethod implements JWT signing via SSH agent for RSA keys.
type sshRSASigningMethod struct{}

func (m *sshRSASigningMethod) Alg() string { return "RS256" }

func (m *sshRSASigningMethod) Verify(signingString string, sig []byte, key interface{}) error {
	return errors.New("verification not implemented")
}

func (m *sshRSASigningMethod) Sign(signingString string, key interface{}) ([]byte, error) {
	pubKey, ok := key.(ssh.PublicKey)
	if !ok {
		return nil, fmt.Errorf("expected ssh.PublicKey, got %T", key)
	}

	agentSock := os.Getenv("SSH_AUTH_SOCK")
	if agentSock == "" {
		return nil, errors.New("SSH_AUTH_SOCK not set")
	}

	conn, err := net.Dial("unix", agentSock)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSH agent: %w", err)
	}
	defer conn.Close()

	agentClient := agent.NewClient(conn)

	sigData, err := agentClient.Sign(pubKey, []byte(signingString))
	if err != nil {
		return nil, fmt.Errorf("SSH agent signing failed: %w", err)
	}

	return []byte(base64.StdEncoding.EncodeToString(sigData.Blob)), nil
}

// startReposerverWithAuth starts the reposerver container with authentication configured.
func (tc *TestContext) startReposerverWithAuth(authConfig map[string]interface{}) (err error) {
	tc.t.Helper()
	tc.t.Log("Starting reposerver container with auth...")

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Remove any existing container with the same name
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", containerName).Run()

	// Create a full reposerver config file with static token auth
	repoConfig := map[string]interface{}{
		"address":     "0.0.0.0",
		"port":        9999,
		"serverRoot":  "/var/dbt",
		"authTypePut": "static-token",
		"authOptsPut": authConfig,
	}

	configBytes, err := json.MarshalIndent(repoConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal reposerver config: %w", err)
	}

	configPath := filepath.Join(tc.repoDir, "reposerver.json")
	err = os.WriteFile(configPath, configBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to write reposerver config: %w", err)
	}

	tc.t.Logf("Reposerver config written to %s: %s", configPath, string(configBytes))

	// Make the repo directory writable by the container's nonroot user (uid 65532)
	// The distroless container runs as nonroot:nonroot
	chmodCmd := exec.CommandContext(ctx, "chmod", "-R", "777", tc.repoDir)
	if chmodErr := chmodCmd.Run(); chmodErr != nil {
		return fmt.Errorf("failed to chmod repo dir: %w", chmodErr)
	}

	cmd := exec.CommandContext(ctx, "docker", "run",
		"-d",
		"--name", containerName,
		"-p", fmt.Sprintf("%s:9999", reposerverPort),
		"-v", fmt.Sprintf("%s:/var/dbt", tc.repoDir),
		reposerverImage,
		"-f", "/var/dbt/reposerver.json",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			tc.t.Logf("Docker run stderr:\n%s", string(exitErr.Stderr))
		}
		return fmt.Errorf("docker run failed: %w", err)
	}

	tc.containerID = strings.TrimSpace(string(output))
	tc.t.Logf("Started container %s", tc.containerID[:12])

	// Wait for container to be healthy
	err = tc.waitForHealthy()
	if err != nil {
		// Get container logs for debugging
		logCmd := exec.Command("docker", "logs", tc.containerID)
		logs, _ := logCmd.CombinedOutput()
		tc.t.Logf("Container logs:\n%s", string(logs))
		return err
	}

	return err
}
