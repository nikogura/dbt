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
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrandDefaults(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "dbt", BrandName, "Default BrandName")
	assert.Equal(t, ".dbt", BrandDir, "Default BrandDir")
	assert.Equal(t, "dbt", BrandBinary, "Default BrandBinary")
	assert.Equal(t, "dbt.json", BrandConfigFile, "Default BrandConfigFile")
	assert.Equal(t, "dbt-tools", BrandToolsPath, "Default BrandToolsPath")
	assert.Equal(t, "DBT", BrandEnvPrefix, "Default BrandEnvPrefix")
}

func TestBrandDerivedPaths(t *testing.T) {
	t.Parallel()

	assert.Equal(t, BrandDir, GetBrandDir())
	assert.Equal(t, BrandDir+"/trust", GetTrustDir())
	assert.Equal(t, BrandDir+"/tools", GetToolDir())
	assert.Equal(t, BrandDir+"/conf", GetConfigDir())
	assert.Equal(t, BrandDir+"/conf/"+BrandConfigFile, GetConfigFilePath())
	assert.Equal(t, BrandDir+"/trust/truststore", GetTruststorePath())
}

func TestBrandEnvVars(t *testing.T) {
	t.Parallel()

	assert.Equal(t, BrandEnvPrefix+"_SERVER", GetServerEnvVar())
	assert.Equal(t, BrandEnvPrefix+"_REPO", GetRepoEnvVar())
	assert.Equal(t, BrandEnvPrefix+"_TOOLS_REPO", GetToolsRepoEnvVar())
	assert.Equal(t, BrandEnvPrefix+"_TRUSTSTORE", GetTruststoreEnvVar())
}

// isAllowlisted checks if a file path matches any allowlisted pattern.
func isAllowlisted(path string, allowlist []string) (allowed bool) {
	for _, allow := range allowlist {
		if strings.Contains(path, allow) {
			allowed = true
			return allowed
		}
	}

	return allowed
}

// usesBrandVar checks if a line references a brand variable (correct usage).
func usesBrandVar(line string) (uses bool) {
	uses = strings.Contains(line, "BrandName") ||
		strings.Contains(line, "BrandBinary") ||
		strings.Contains(line, "BrandDir") ||
		strings.Contains(line, "BrandToolsPath")
	return uses
}

// scanFileForHardcodedBrand scans a single file for hardcoded "dbt" in format strings.
func scanFileForHardcodedBrand(path string, patterns []*regexp.Regexp) (violations []string, err error) {
	file, openErr := os.Open(path)
	if openErr != nil {
		err = openErr
		return violations, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}

		for _, pattern := range patterns {
			if pattern.MatchString(line) && !usesBrandVar(line) {
				violations = append(violations, fmt.Sprintf("%s:%d: %s", path, lineNum, trimmed))
			}
		}
	}

	err = scanner.Err()
	return violations, err
}

// TestNoHardcodedDbtInFormatStrings scans Go source files for hardcoded "dbt"
// in format strings that should use brand variables instead.
//
// This prevents regressions like https://github.com/nikogura/dynamic-binary-toolkit/issues/28.
func TestNoHardcodedDbtInFormatStrings(t *testing.T) {
	t.Parallel()

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:log|fmt)\.\w+f?\(.*"[^"]*\bdbt\b[^"]*"`),
		regexp.MustCompile(`Fprintf\(\w+,\s*"[^"]*\bdbt\b[^"]*"`),
	}

	allowlist := []string{
		"brand.go", "brand_test.go", "_test.go", "dbt_setup_test",
	}

	roots := []string{
		filepath.Join("..", "..", "cmd"),
		filepath.Join("..", "..", "pkg"),
	}

	var allViolations []string

	for _, root := range roots {
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) (err error) {
			if walkErr != nil {
				err = walkErr
				return err
			}

			if info.IsDir() || !strings.HasSuffix(path, ".go") || isAllowlisted(path, allowlist) {
				return err
			}

			fileViolations, scanErr := scanFileForHardcodedBrand(path, patterns)
			if scanErr != nil {
				err = scanErr
				return err
			}

			allViolations = append(allViolations, fileViolations...)
			return err
		})
		if walkErr != nil {
			t.Logf("Warning: could not walk %s: %v", root, walkErr)
		}
	}

	assert.Empty(t, allViolations,
		"Found hardcoded 'dbt' in format strings that should use brand variables.\n"+
			"See https://github.com/nikogura/dynamic-binary-toolkit/issues/28\n"+
			"Violations:\n"+strings.Join(allViolations, "\n"))
}
