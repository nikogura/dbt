// Copyright 2025 Nik Ogura <nik.ogura@gmail.com>
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

package installer

// Installer brand variables — override at build time for organizational rebranding.
//
// Build with:
//
//	go build -ldflags "\
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandName=myorg \
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandDir=.myorg \
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandBinary=myorg \
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandConfigFile=myorg.json \
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandToolsPath=myorg-tools \
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandOIDCClientID=myorg \
//	  -X github.com/nikogura/dbt/cmd/dbt-installer/installer.BrandOIDCSSHClientID=myorg-ssh"

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

// BrandOIDCClientID is the default OIDC client ID (e.g. "dbt").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandOIDCClientID = "dbt"

// BrandOIDCSSHClientID is the OIDC client ID for SSH connector (e.g. "dbt-ssh").
//
//nolint:gochecknoglobals // Must be var for ldflags injection.
var BrandOIDCSSHClientID = "dbt-ssh"
