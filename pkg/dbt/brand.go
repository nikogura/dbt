// Copyright © 2024 Nik Ogura <nik.ogura@gmail.com>
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

// Brand variables — override at build time for organizational rebranding.
//
// Build with:
//
//	go build -ldflags "\
//	  -X github.com/nikogura/dbt/pkg/dbt.BrandName=myorg \
//	  -X github.com/nikogura/dbt/pkg/dbt.BrandDir=.myorg \
//	  -X github.com/nikogura/dbt/pkg/dbt.BrandBinary=myorg \
//	  -X github.com/nikogura/dbt/pkg/dbt.BrandConfigFile=myorg.json \
//	  -X github.com/nikogura/dbt/pkg/dbt.BrandToolsPath=myorg-tools \
//	  -X github.com/nikogura/dbt/pkg/dbt.BrandEnvPrefix=MYORG"

// BrandName is the human-readable name of the tool.
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandName = "dbt"

// BrandDir is the dot-directory name under $HOME (e.g. ".dbt").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandDir = ".dbt"

// BrandBinary is the expected binary filename (e.g. "dbt").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandBinary = "dbt"

// BrandConfigFile is the config filename (e.g. "dbt.json").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandConfigFile = "dbt.json"

// BrandToolsPath is the repo path segment for tools (e.g. "dbt-tools").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandToolsPath = "dbt-tools"

// BrandEnvPrefix is the prefix for environment variables (e.g. "DBT").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandEnvPrefix = "DBT"

// Derived paths — computed from brand vars at init time.

// GetBrandDir returns the dot-directory name.
func GetBrandDir() (dir string) {
	dir = BrandDir
	return dir
}

// GetTrustDir returns the trust directory path relative to home.
func GetTrustDir() (dir string) {
	dir = BrandDir + "/trust"
	return dir
}

// GetToolDir returns the tool directory path relative to home.
func GetToolDir() (dir string) {
	dir = BrandDir + "/tools"
	return dir
}

// GetConfigDir returns the config directory path relative to home.
func GetConfigDir() (dir string) {
	dir = BrandDir + "/conf"
	return dir
}

// GetConfigFilePath returns the config file path relative to home.
func GetConfigFilePath() (path string) {
	path = BrandDir + "/conf/" + BrandConfigFile
	return path
}

// GetTruststorePath returns the truststore file path relative to home.
func GetTruststorePath() (path string) {
	path = BrandDir + "/trust/truststore"
	return path
}

// GetServerEnvVar returns the environment variable name for server selection.
func GetServerEnvVar() (envVar string) {
	envVar = BrandEnvPrefix + "_SERVER"
	return envVar
}

// GetRepoEnvVar returns the environment variable name for the repo URL.
func GetRepoEnvVar() (envVar string) {
	envVar = BrandEnvPrefix + "_REPO"
	return envVar
}

// GetToolsRepoEnvVar returns the environment variable name for the tools repo URL.
func GetToolsRepoEnvVar() (envVar string) {
	envVar = BrandEnvPrefix + "_TOOLS_REPO"
	return envVar
}

// GetTruststoreEnvVar returns the environment variable name for the truststore URL.
func GetTruststoreEnvVar() (envVar string) {
	envVar = BrandEnvPrefix + "_TRUSTSTORE"
	return envVar
}
