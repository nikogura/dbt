// Copyright © 2026 Nik Ogura <nik.ogura@gmail.com>
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
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerfilesUseTargetarchForMultiArch scans all Dockerfiles for hardcoded
// GOARCH values and verifies they use the buildx ARG TARGETARCH pattern instead.
//
// This prevents regressions like https://github.com/nikogura/dynamic-binary-toolkit/issues/29
// where hardcoded GOARCH=amd64 prevents multi-arch Docker builds.
func TestDockerfilesUseTargetarchForMultiArch(t *testing.T) {
	t.Parallel()

	dockerfilesDir := filepath.Join("..", "..", "dockerfiles")

	var dockerfiles []string
	walkErr := filepath.Walk(dockerfilesDir, func(path string, info os.FileInfo, walkErr error) (err error) {
		if walkErr != nil {
			err = walkErr
			return err
		}

		if !info.IsDir() && strings.Contains(info.Name(), "Dockerfile") {
			dockerfiles = append(dockerfiles, path)
		}

		return err
	})
	require.NoError(t, walkErr, "Failed to walk dockerfiles directory")
	require.NotEmpty(t, dockerfiles, "No Dockerfiles found in %s", dockerfilesDir)

	for _, df := range dockerfiles {
		t.Run(filepath.Base(filepath.Dir(df)), func(t *testing.T) {
			t.Parallel()
			checkDockerfileMultiArch(t, df)
		})
	}
}

// dockerfileAnalysis holds the results of scanning a Dockerfile.
type dockerfileAnalysis struct {
	violations       []string
	hasTargetarch    bool
	hasTargetos      bool
	hasBuildplatform bool
}

// analyzeDockerfileLine checks a single line for multi-arch issues.
func analyzeDockerfileLine(path string, lineNum int, trimmed string, result *dockerfileAnalysis) {
	// Check for hardcoded GOARCH.
	if strings.Contains(trimmed, "GOARCH=") && !strings.Contains(trimmed, "$TARGETARCH") && !strings.Contains(trimmed, "${TARGETARCH}") {
		result.violations = append(result.violations, fmt.Sprintf("%s:%d: hardcoded GOARCH — use $TARGETARCH: %s", path, lineNum, trimmed))
	}

	// Track expected ARG declarations.
	if strings.Contains(trimmed, "ARG TARGETARCH") {
		result.hasTargetarch = true
	}

	if strings.Contains(trimmed, "ARG TARGETOS") {
		result.hasTargetos = true
	}

	if strings.Contains(trimmed, "$BUILDPLATFORM") || strings.Contains(trimmed, "${BUILDPLATFORM}") {
		result.hasBuildplatform = true
	}
}

// scanDockerfile reads a Dockerfile and returns the analysis results.
func scanDockerfile(path string) (result dockerfileAnalysis, err error) {
	file, openErr := os.Open(path)
	if openErr != nil {
		err = openErr
		return result, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		trimmed := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		analyzeDockerfileLine(path, lineNum, trimmed, &result)
	}

	err = scanner.Err()
	return result, err
}

// checkDockerfileMultiArch verifies a single Dockerfile follows multi-arch best practices.
func checkDockerfileMultiArch(t *testing.T, dockerfilePath string) {
	t.Helper()

	result, scanErr := scanDockerfile(dockerfilePath)
	require.NoError(t, scanErr)

	hasGoBuild := fileContains(t, dockerfilePath, "go build")
	if hasGoBuild {
		assert.Empty(t, result.violations, "Dockerfile has hardcoded architecture in go build commands")
		assert.True(t, result.hasTargetarch, "%s: should declare ARG TARGETARCH", dockerfilePath)
		assert.True(t, result.hasTargetos, "%s: should declare ARG TARGETOS", dockerfilePath)
		assert.True(t, result.hasBuildplatform, "%s: builder stage should use --platform=$BUILDPLATFORM", dockerfilePath)
	}
}

// fileContains checks if a file contains a given string.
func fileContains(t *testing.T, path string, needle string) (found bool) {
	t.Helper()

	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)

	found = strings.Contains(string(data), needle)
	return found
}
