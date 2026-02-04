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

package testfixtures

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

// Artifact represents a test artifact with its content, checksum, and signature.
type Artifact struct {
	Binary    []byte
	Checksum  string
	Signature string
}

// ArtifactSet contains artifacts for both test versions.
type ArtifactSet struct {
	OldVersion string // 3.0.2
	NewVersion string // 3.3.4
	Old        Artifact
	New        Artifact
}

// GetDbtArtifacts returns the dbt binary artifacts.
func GetDbtArtifacts() (artifacts ArtifactSet) {
	artifacts = ArtifactSet{
		OldVersion: "3.0.2",
		NewVersion: "3.3.4",
		Old: Artifact{
			Binary:    Dbt302Binary,
			Checksum:  strings.TrimSpace(Dbt302Checksum),
			Signature: Dbt302Signature,
		},
		New: Artifact{
			Binary:    Dbt334Binary,
			Checksum:  strings.TrimSpace(Dbt334Checksum),
			Signature: Dbt334Signature,
		},
	}
	return artifacts
}

// GetCatalogArtifacts returns the catalog tool artifacts.
func GetCatalogArtifacts() (artifacts ArtifactSet) {
	artifacts = ArtifactSet{
		OldVersion: "3.0.2",
		NewVersion: "3.3.4",
		Old: Artifact{
			Binary:    Catalog302Binary,
			Checksum:  strings.TrimSpace(Catalog302Checksum),
			Signature: Catalog302Signature,
		},
		New: Artifact{
			Binary:    Catalog334Binary,
			Checksum:  strings.TrimSpace(Catalog334Checksum),
			Signature: Catalog334Signature,
		},
	}
	return artifacts
}

// SetupTestRepo creates the test repository structure in the given directory.
// Returns the truststore content for use in tests.
func SetupTestRepo(tmpDir string) (truststoreContent string, err error) {
	dbtRoot := filepath.Join(tmpDir, "repo", "dbt")
	toolRoot := filepath.Join(tmpDir, "repo", "dbt-tools")

	// Create directory structure
	dirs := []string{
		filepath.Join(dbtRoot, "3.0.2", "linux", "amd64"),
		filepath.Join(dbtRoot, "3.3.4", "linux", "amd64"),
		filepath.Join(dbtRoot, "3.7.0", "linux", "amd64"),
		filepath.Join(toolRoot, "catalog", "3.0.2", "linux", "amd64"),
		filepath.Join(toolRoot, "catalog", "3.3.4", "linux", "amd64"),
		filepath.Join(toolRoot, "catalog", "3.7.0", "linux", "amd64"),
	}

	for _, dir := range dirs {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create directory %s", dir)
			return truststoreContent, err
		}
	}

	// Write truststore
	truststoreContent = Truststore
	truststorePath := filepath.Join(dbtRoot, "truststore")
	err = os.WriteFile(truststorePath, []byte(truststoreContent), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write truststore")
		return truststoreContent, err
	}

	// Write dbt binaries for both versions
	err = writeArtifact(dbtRoot, "3.0.2", "dbt", Dbt302Binary, Dbt302Checksum, Dbt302Signature)
	if err != nil {
		return truststoreContent, err
	}

	err = writeArtifact(dbtRoot, "3.3.4", "dbt", Dbt334Binary, Dbt334Checksum, Dbt334Signature)
	if err != nil {
		return truststoreContent, err
	}

	err = writeArtifact(dbtRoot, "3.7.0", "dbt", Dbt370Binary, Dbt370Checksum, Dbt370Signature)
	if err != nil {
		return truststoreContent, err
	}

	// Write catalog binaries for all versions
	err = writeCatalogArtifact(toolRoot, "3.0.2",
		Catalog302Binary, Catalog302Checksum, Catalog302Signature,
		Catalog302Description, Catalog302DescriptionSig)
	if err != nil {
		return truststoreContent, err
	}

	err = writeCatalogArtifact(toolRoot, "3.3.4",
		Catalog334Binary, Catalog334Checksum, Catalog334Signature,
		Catalog334Description, Catalog334DescriptionSig)
	if err != nil {
		return truststoreContent, err
	}

	err = writeCatalogArtifact(toolRoot, "3.7.0",
		Catalog370Binary, Catalog370Checksum, Catalog370Signature,
		Catalog370Description, Catalog370DescriptionSig)
	if err != nil {
		return truststoreContent, err
	}

	// Write install scripts
	err = writeInstallScripts(dbtRoot)
	if err != nil {
		return truststoreContent, err
	}

	return truststoreContent, err
}

func writeArtifact(root, version, name string, binary []byte, checksum, signature string) (err error) {
	dir := filepath.Join(root, version, "linux", "amd64")

	binaryPath := filepath.Join(dir, name)
	err = os.WriteFile(binaryPath, binary, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", binaryPath)
		return err
	}

	checksumPath := fmt.Sprintf("%s.sha256", binaryPath)
	err = os.WriteFile(checksumPath, []byte(strings.TrimSpace(checksum)), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", checksumPath)
		return err
	}

	signaturePath := fmt.Sprintf("%s.asc", binaryPath)
	err = os.WriteFile(signaturePath, []byte(signature), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", signaturePath)
		return err
	}

	return err
}

func writeCatalogArtifact(toolRoot, version string, binary []byte, checksum, signature, description, descriptionSig string) (err error) {
	baseDir := filepath.Join(toolRoot, "catalog", version)
	binaryDir := filepath.Join(baseDir, "linux", "amd64")

	// Write description files
	descPath := filepath.Join(baseDir, "description.txt")
	err = os.WriteFile(descPath, []byte(description), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", descPath)
		return err
	}

	descSigPath := fmt.Sprintf("%s.asc", descPath)
	err = os.WriteFile(descSigPath, []byte(descriptionSig), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", descSigPath)
		return err
	}

	// Write binary
	binaryPath := filepath.Join(binaryDir, "catalog")
	err = os.WriteFile(binaryPath, binary, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", binaryPath)
		return err
	}

	checksumPath := fmt.Sprintf("%s.sha256", binaryPath)
	err = os.WriteFile(checksumPath, []byte(strings.TrimSpace(checksum)), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", checksumPath)
		return err
	}

	signaturePath := fmt.Sprintf("%s.asc", binaryPath)
	err = os.WriteFile(signaturePath, []byte(signature), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", signaturePath)
		return err
	}

	return err
}

func writeInstallScripts(dbtRoot string) (err error) {
	// install_dbt.sh
	installPath := filepath.Join(dbtRoot, "install_dbt.sh")
	err = os.WriteFile(installPath, InstallDbtScript, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", installPath)
		return err
	}

	err = os.WriteFile(installPath+".sha256", []byte(strings.TrimSpace(InstallDbtScriptChecksum)), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s.sha256", installPath)
		return err
	}

	err = os.WriteFile(installPath+".asc", []byte(InstallDbtScriptSignature), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s.asc", installPath)
		return err
	}

	// install_dbt_mac_keychain.sh
	macInstallPath := filepath.Join(dbtRoot, "install_dbt_mac_keychain.sh")
	err = os.WriteFile(macInstallPath, InstallDbtMacScript, 0755)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", macInstallPath)
		return err
	}

	err = os.WriteFile(macInstallPath+".sha256", []byte(strings.TrimSpace(InstallDbtMacScriptChecksum)), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s.sha256", macInstallPath)
		return err
	}

	err = os.WriteFile(macInstallPath+".asc", []byte(InstallDbtMacScriptSignature), 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s.asc", macInstallPath)
		return err
	}

	return err
}
