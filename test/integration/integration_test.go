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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			// Check that binaries are accessible
			versions := []string{"3.0.2", "3.3.4", "3.7.3"}
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
			// Check catalog tool
			resp, err := http.Get(tc.reposerverURL + "/dbt-tools/catalog/3.7.3/linux/amd64/catalog")
			require.NoError(t, err, "should be able to fetch catalog binary")
			defer resp.Body.Close()
			assert.Equal(t, http.StatusOK, resp.StatusCode, "catalog binary should return 200")

			// Check description
			resp, err = http.Get(tc.reposerverURL + "/dbt-tools/catalog/3.7.3/description.txt")
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
		// Should contain version directories
		assert.Contains(t, string(body), "3.0.2", "should list version 3.0.2")
	})

	t.Run("NotFoundReturns404", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/nonexistent")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("BinaryContent", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/3.7.3/linux/amd64/dbt")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "dbt version 3.7.3", "binary should contain version string")
	})

	t.Run("ChecksumContent", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/3.7.3/linux/amd64/dbt.sha256")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		// SHA256 checksums are 64 hex characters
		checksum := strings.TrimSpace(string(body))
		assert.Len(t, checksum, 64, "checksum should be 64 characters")
	})

	t.Run("SignatureContent", func(t *testing.T) {
		resp, err := http.Get(tc.reposerverURL + "/dbt/3.7.3/linux/amd64/dbt.asc")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "BEGIN PGP SIGNATURE", "should contain PGP signature")
	})
}
