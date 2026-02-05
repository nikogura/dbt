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

// Package testfixtures provides pre-built test artifacts for dbt tests.
// These fixtures replace the slow gomason compilation process with static files.
package testfixtures

import (
	_ "embed"
)

// PublicKey is the GPG test key (public key for signature verification).
//
//go:embed gpg/public-key.asc
var PublicKey string

// Dbt302Binary is the DBT binary for version 3.0.2.
//
//go:embed repo/dbt/3.0.2/linux/amd64/dbt
var Dbt302Binary []byte

//go:embed repo/dbt/3.0.2/linux/amd64/dbt.sha256
var Dbt302Checksum string

//go:embed repo/dbt/3.0.2/linux/amd64/dbt.asc
var Dbt302Signature string

// Dbt334Binary is the DBT binary for version 3.3.4.
//
//go:embed repo/dbt/3.3.4/linux/amd64/dbt
var Dbt334Binary []byte

//go:embed repo/dbt/3.3.4/linux/amd64/dbt.sha256
var Dbt334Checksum string

//go:embed repo/dbt/3.3.4/linux/amd64/dbt.asc
var Dbt334Signature string

// Dbt371Binary is the DBT binary for version 3.7.0.
//
//go:embed repo/dbt/3.7.1/linux/amd64/dbt
var Dbt371Binary []byte

//go:embed repo/dbt/3.7.1/linux/amd64/dbt.sha256
var Dbt371Checksum string

//go:embed repo/dbt/3.7.1/linux/amd64/dbt.asc
var Dbt371Signature string

// Catalog302Binary is the catalog binary for version 3.0.2.
//
//go:embed repo/dbt-tools/catalog/3.0.2/linux/amd64/catalog
var Catalog302Binary []byte

//go:embed repo/dbt-tools/catalog/3.0.2/linux/amd64/catalog.sha256
var Catalog302Checksum string

//go:embed repo/dbt-tools/catalog/3.0.2/linux/amd64/catalog.asc
var Catalog302Signature string

//go:embed repo/dbt-tools/catalog/3.0.2/description.txt
var Catalog302Description string

//go:embed repo/dbt-tools/catalog/3.0.2/description.txt.asc
var Catalog302DescriptionSig string

// Catalog334Binary is the catalog binary for version 3.3.4.
//
//go:embed repo/dbt-tools/catalog/3.3.4/linux/amd64/catalog
var Catalog334Binary []byte

//go:embed repo/dbt-tools/catalog/3.3.4/linux/amd64/catalog.sha256
var Catalog334Checksum string

//go:embed repo/dbt-tools/catalog/3.3.4/linux/amd64/catalog.asc
var Catalog334Signature string

//go:embed repo/dbt-tools/catalog/3.3.4/description.txt
var Catalog334Description string

//go:embed repo/dbt-tools/catalog/3.3.4/description.txt.asc
var Catalog334DescriptionSig string

// Catalog371Binary is the catalog binary for version 3.7.0.
//
//go:embed repo/dbt-tools/catalog/3.7.1/linux/amd64/catalog
var Catalog371Binary []byte

//go:embed repo/dbt-tools/catalog/3.7.1/linux/amd64/catalog.sha256
var Catalog371Checksum string

//go:embed repo/dbt-tools/catalog/3.7.1/linux/amd64/catalog.asc
var Catalog371Signature string

//go:embed repo/dbt-tools/catalog/3.7.1/description.txt
var Catalog371Description string

//go:embed repo/dbt-tools/catalog/3.7.1/description.txt.asc
var Catalog371DescriptionSig string

// Truststore is the GPG truststore (same as public key).
//
//go:embed repo/dbt/truststore
var Truststore string

// InstallDbtScript is the dbt installation script.
//
//go:embed repo/dbt/install_dbt.sh
var InstallDbtScript []byte

//go:embed repo/dbt/install_dbt.sh.sha256
var InstallDbtScriptChecksum string

//go:embed repo/dbt/install_dbt.sh.asc
var InstallDbtScriptSignature string

//go:embed repo/dbt/install_dbt_mac_keychain.sh
var InstallDbtMacScript []byte

//go:embed repo/dbt/install_dbt_mac_keychain.sh.sha256
var InstallDbtMacScriptChecksum string

//go:embed repo/dbt/install_dbt_mac_keychain.sh.asc
var InstallDbtMacScriptSignature string
