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

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/net/html"
)

// connectorSSH is the SSH-OIDC connector identifier.
const connectorSSH = "ssh"

// Run is the main entry point for the installer.
func Run(config *Config) {
	// Validate configuration
	validateErr := config.Validate()
	if validateErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", validateErr)
		os.Exit(1)
	}

	// Fill in defaults
	config.DeriveDefaults()

	installer := &Installer{
		config: config,
	}

	installErr := installer.Install()
	if installErr != nil {
		fmt.Fprintf(os.Stderr, "\nError: %s\n", installErr)
		os.Exit(1)
	}
}

// Installer handles the dbt installation process.
type Installer struct {
	config     *Config
	authClient *AuthClient
	token      string
	isS3       bool
	s3Bucket   string
	s3Region   string
	s3Client   *s3.S3
}

// Install performs the full installation process.
func (i *Installer) Install() (err error) {
	fmt.Println()
	fmt.Println("dbt installer")
	fmt.Println("=============")
	fmt.Printf("Server: %s\n", i.config.ServerURL)
	fmt.Printf("Name:   %s\n", i.config.ServerName)
	fmt.Printf("OS:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()

	// Detect S3 URL
	i.isS3 = strings.HasPrefix(i.config.ServerURL, "s3://")
	if i.isS3 {
		setupErr := i.setupS3()
		if setupErr != nil {
			err = fmt.Errorf("failed to setup S3: %w", setupErr)
			return err
		}
	}

	// Determine username for OIDC (if applicable)
	var username string
	if i.config.IssuerURL != "" && i.config.ConnectorID == connectorSSH {
		username, err = i.promptForUsername()
		if err != nil {
			return err
		}
	}

	// Authenticate if OIDC is configured (for non-S3 servers)
	if !i.isS3 && i.config.IssuerURL != "" {
		i.authClient = NewAuthClient(i.config, username)
		i.token, err = i.authClient.GetToken(context.Background())
		if err != nil {
			return err
		}
	}

	// Get latest version
	fmt.Println("Fetching latest version...")
	version, versionErr := i.fetchLatestVersion()
	if versionErr != nil {
		err = fmt.Errorf("failed to fetch latest version: %w", versionErr)
		return err
	}
	fmt.Printf("  Latest version: %s\n", version)

	// Download dbt binary
	fmt.Println("Downloading dbt...")
	binaryPath, downloadErr := i.downloadDbt(version)
	if downloadErr != nil {
		err = fmt.Errorf("failed to download dbt: %w", downloadErr)
		return err
	}

	// Install binary
	fmt.Println("Installing...")
	installPath, installErr := i.installBinary(binaryPath)
	if installErr != nil {
		err = fmt.Errorf("failed to install dbt: %w", installErr)
		return err
	}
	fmt.Printf("  Installed to: %s\n", installPath)

	// Configure
	fmt.Println("Configuring...")
	configErr := i.configure(username)
	if configErr != nil {
		err = fmt.Errorf("failed to configure dbt: %w", configErr)
		return err
	}

	// Update shell profile and print completion message
	i.printCompletionMessage(installPath)

	return err
}

// printCompletionMessage prints the post-installation instructions.
func (i *Installer) printCompletionMessage(installPath string) {
	pathUpdated := i.updateShellProfile(installPath)

	fmt.Println()
	fmt.Println("Installation complete!")
	fmt.Println()

	installDir := filepath.Dir(installPath)
	if !isInPath(installDir) {
		fmt.Println("To start using dbt, either:")
		fmt.Println()
		fmt.Println("  1. Open a new terminal, or")
		fmt.Printf("  2. Run: export PATH=\"%s:$PATH\"\n", installDir)
		fmt.Println()
	} else if pathUpdated {
		fmt.Println("Restart your terminal or run: source ~/.bashrc (or ~/.zshrc)")
		fmt.Println()
	}

	fmt.Println("Then run:")
	fmt.Println()
	fmt.Println("  dbt -- catalog list")
	fmt.Println()
}

// setupS3 configures S3 client for S3-based installations.
func (i *Installer) setupS3() (err error) {
	// Parse S3 URL: s3://bucket-name/optional/path
	urlWithoutScheme := strings.TrimPrefix(i.config.ServerURL, "s3://")
	parts := strings.SplitN(urlWithoutScheme, "/", 2)
	i.s3Bucket = parts[0]

	// Create session with shared config enabled (reads ~/.aws/config for SSO, profiles, etc.)
	sess, sessErr := session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})
	if sessErr != nil {
		err = fmt.Errorf("failed to create AWS session: %w", sessErr)
		return err
	}

	// Get bucket region
	region, regionErr := s3.New(sess).GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(i.s3Bucket),
	})
	if regionErr != nil {
		// Default to us-east-1 if we can't detect
		i.s3Region = "us-east-1"
	} else if region.LocationConstraint == nil || *region.LocationConstraint == "" {
		i.s3Region = "us-east-1"
	} else {
		i.s3Region = *region.LocationConstraint
	}

	// Create S3 client with correct region and shared config
	sess, sessErr = session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
		Config: aws.Config{
			Region: aws.String(i.s3Region),
		},
	})
	if sessErr != nil {
		err = fmt.Errorf("failed to create AWS session: %w", sessErr)
		return err
	}

	i.s3Client = s3.New(sess)

	// Update config URL to HTTPS endpoint for dbt config
	i.config.ServerURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", i.s3Bucket, i.s3Region)

	// Also convert ToolsURL if it's an S3 URL
	if strings.HasPrefix(i.config.ToolsURL, "s3://") {
		toolsBucket := strings.TrimPrefix(i.config.ToolsURL, "s3://")
		toolsBucket = strings.Split(toolsBucket, "/")[0] // Remove any path
		i.config.ToolsURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", toolsBucket, i.s3Region)
	}

	fmt.Printf("  S3 bucket: %s (region: %s)\n", i.s3Bucket, i.s3Region)

	return err
}

// promptForUsername prompts the user for their OIDC username.
func (i *Installer) promptForUsername() (username string, err error) {
	// Get current system username as default (equivalent to whoami)
	defaultUsername := os.Getenv("USER")
	if defaultUsername == "" {
		defaultUsername = os.Getenv("USERNAME") // Windows fallback
	}
	if defaultUsername == "" {
		// Try os/user as final fallback
		currentUser, userErr := user.Current()
		if userErr == nil {
			defaultUsername = currentUser.Username
		}
	}

	fmt.Println()
	fmt.Println("SSH-OIDC requires a username for authentication.")
	fmt.Println("This should match your identity in the OIDC provider (Dex).")
	fmt.Println()
	fmt.Printf("Enter OIDC username [%s]: ", defaultUsername)

	reader := bufio.NewReader(os.Stdin)
	input, readErr := reader.ReadString('\n')
	if readErr != nil {
		err = fmt.Errorf("failed to read input: %w", readErr)
		return username, err
	}

	username = strings.TrimSpace(input)
	if username == "" {
		username = defaultUsername
	}

	return username, err
}

// fetchLatestVersion retrieves the latest dbt version from the server.
func (i *Installer) fetchLatestVersion() (version string, err error) {
	if i.isS3 {
		// S3 repos have a "latest" file at the root
		version, err = i.fetchFromS3("latest")
		version = strings.TrimSpace(version)
		return version, err
	}

	// HTTP repos: fetch directory listing and parse for versions
	versions, listErr := i.listVersionsFromHTTP(i.config.ServerURL + "/dbt/")
	if listErr != nil {
		err = fmt.Errorf("failed to list versions: %w", listErr)
		return version, err
	}

	if len(versions) == 0 {
		err = errors.New("no versions found in repository")
		return version, err
	}

	version = latestVersion(versions)
	return version, err
}

// listVersionsFromHTTP fetches a directory listing and extracts semantic versions.
func (i *Installer) listVersionsFromHTTP(dirURL string) (versions []string, err error) {
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, dirURL, nil)
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return versions, err
	}

	if i.token != "" {
		req.Header.Set("Authorization", "Bearer "+i.token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("request failed: %w", respErr)
		return versions, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP %d", resp.StatusCode)
		return versions, err
	}

	// Parse HTML for version links
	versions = parseVersionsFromHTML(resp.Body)
	return versions, err
}

// parseVersionsFromHTML extracts semantic version strings from HTML anchor tags.
func parseVersionsFromHTML(body io.Reader) (versions []string) {
	semverPattern := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	tokenizer := html.NewTokenizer(body)

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			return versions
		}
		if tt == html.StartTagToken {
			versions = extractVersionFromToken(tokenizer, semverPattern, versions)
		}
	}
}

// extractVersionFromToken checks if a token is an anchor with a semver href.
func extractVersionFromToken(tokenizer *html.Tokenizer, pattern *regexp.Regexp, existing []string) (versions []string) {
	versions = existing
	t := tokenizer.Token()
	if t.Data != "a" {
		return versions
	}
	for _, attr := range t.Attr {
		if attr.Key == "href" {
			version := strings.TrimRight(attr.Val, "/")
			if pattern.MatchString(version) {
				versions = append(versions, version)
			}
		}
	}
	return versions
}

// latestVersion returns the highest semantic version from a list.
func latestVersion(versions []string) (latest string) {
	for _, v := range versions {
		if latest == "" {
			latest = v
		} else if versionGreater(v, latest) {
			latest = v
		}
	}
	return latest
}

// versionGreater returns true if version a is greater than version b.
func versionGreater(a, b string) (greater bool) {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	for i := range 3 {
		aNum := parseVersionPart(aParts, i)
		bNum := parseVersionPart(bParts, i)
		if aNum > bNum {
			greater = true
			return greater
		}
		if aNum < bNum {
			return greater
		}
	}
	return greater
}

// parseVersionPart safely parses a version part as an integer.
func parseVersionPart(parts []string, index int) (num int) {
	if index < len(parts) {
		parsed, parseErr := strconv.Atoi(parts[index])
		if parseErr == nil {
			num = parsed
		}
	}
	return num
}

// fetchFromS3 retrieves content from S3.
func (i *Installer) fetchFromS3(key string) (content string, err error) {
	result, getErr := i.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(i.s3Bucket),
		Key:    aws.String(key),
	})
	if getErr != nil {
		err = fmt.Errorf("S3 get failed: %w", getErr)
		return content, err
	}
	defer result.Body.Close()

	data, readErr := io.ReadAll(result.Body)
	if readErr != nil {
		err = fmt.Errorf("failed to read S3 response: %w", readErr)
		return content, err
	}

	content = string(data)
	return content, err
}

// fetchFromHTTP retrieves content from HTTP with optional auth.
func (i *Installer) fetchFromHTTP(fetchURL string) (content string, err error) {
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, fetchURL, nil)
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return content, err
	}

	if i.token != "" {
		req.Header.Set("Authorization", "Bearer "+i.token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("request failed: %w", respErr)
		return content, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP %d", resp.StatusCode)
		return content, err
	}

	data, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		err = fmt.Errorf("failed to read response: %w", readErr)
		return content, err
	}

	content = string(data)
	return content, err
}

// downloadDbt downloads the dbt binary for the current platform.
func (i *Installer) downloadDbt(version string) (binaryPath string, err error) {
	// Map architecture
	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
		// These architectures use their native names
	default:
		// Keep as is for other architectures
	}

	// Build download path
	binaryKey := fmt.Sprintf("%s/%s/%s/dbt", version, runtime.GOOS, arch)
	checksumKey := binaryKey + ".sha256"

	// Create temp file
	tmpDir, tmpErr := os.MkdirTemp("", "dbt-installer-*")
	if tmpErr != nil {
		err = fmt.Errorf("failed to create temp dir: %w", tmpErr)
		return binaryPath, err
	}

	binaryPath = filepath.Join(tmpDir, "dbt")

	// Download binary
	if i.isS3 {
		err = i.downloadFileFromS3(binaryKey, binaryPath)
	} else {
		// HTTP repos have the structure: <base>/dbt/<version>/<os>/<arch>/dbt
		downloadURL := i.config.ServerURL + "/dbt/" + binaryKey
		err = i.downloadFileFromHTTP(downloadURL, binaryPath)
	}
	if err != nil {
		return binaryPath, err
	}

	// Verify checksum
	verifyErr := i.verifyChecksum(binaryPath, checksumKey)
	if verifyErr != nil {
		fmt.Printf("  Warning: checksum verification failed: %s\n", verifyErr)
		// Continue anyway - checksum might not be available
	} else {
		fmt.Println("  Checksum verified")
	}

	return binaryPath, err
}

// downloadFileFromS3 downloads a file from S3.
func (i *Installer) downloadFileFromS3(key, destPath string) (err error) {
	result, getErr := i.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(i.s3Bucket),
		Key:    aws.String(key),
	})
	if getErr != nil {
		err = fmt.Errorf("S3 download failed: %w", getErr)
		return err
	}
	defer result.Body.Close()

	outFile, createErr := os.Create(destPath)
	if createErr != nil {
		err = fmt.Errorf("failed to create file: %w", createErr)
		return err
	}
	defer outFile.Close()

	_, copyErr := io.Copy(outFile, result.Body)
	if copyErr != nil {
		err = fmt.Errorf("failed to write file: %w", copyErr)
		return err
	}

	return err
}

// downloadFileFromHTTP downloads a file from HTTP with optional auth.
func (i *Installer) downloadFileFromHTTP(url, destPath string) (err error) {
	req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if reqErr != nil {
		err = fmt.Errorf("failed to create request: %w", reqErr)
		return err
	}

	if i.token != "" {
		req.Header.Set("Authorization", "Bearer "+i.token)
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, respErr := client.Do(req)
	if respErr != nil {
		err = fmt.Errorf("download failed: %w", respErr)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP %d", resp.StatusCode)
		return err
	}

	outFile, createErr := os.Create(destPath)
	if createErr != nil {
		err = fmt.Errorf("failed to create file: %w", createErr)
		return err
	}
	defer outFile.Close()

	_, copyErr := io.Copy(outFile, resp.Body)
	if copyErr != nil {
		err = fmt.Errorf("failed to write file: %w", copyErr)
		return err
	}

	return err
}

// verifyChecksum verifies the SHA256 checksum of a file.
func (i *Installer) verifyChecksum(filePath, checksumKey string) (err error) {
	// Fetch expected checksum
	var expectedChecksum string
	if i.isS3 {
		expectedChecksum, err = i.fetchFromS3(checksumKey)
	} else {
		// HTTP repos have the structure: <base>/dbt/<checksumKey>
		expectedChecksum, err = i.fetchFromHTTP(i.config.ServerURL + "/dbt/" + checksumKey)
	}
	if err != nil {
		return err
	}

	expectedChecksum = strings.TrimSpace(strings.Fields(expectedChecksum)[0])

	// Calculate actual checksum
	file, openErr := os.Open(filePath)
	if openErr != nil {
		err = fmt.Errorf("failed to open file: %w", openErr)
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	_, copyErr := io.Copy(hasher, file)
	if copyErr != nil {
		err = fmt.Errorf("failed to hash file: %w", copyErr)
		return err
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))

	if actualChecksum != expectedChecksum {
		err = fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
		return err
	}

	return err
}

// installBinary installs the dbt binary to the user's local bin directory.
func (i *Installer) installBinary(sourcePath string) (installPath string, err error) {
	// Determine install directory
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		err = fmt.Errorf("failed to get home directory: %w", homeErr)
		return installPath, err
	}

	installDir := filepath.Join(homeDir, ".local", "bin")
	mkdirErr := os.MkdirAll(installDir, 0755)
	if mkdirErr != nil {
		err = fmt.Errorf("failed to create install directory: %w", mkdirErr)
		return installPath, err
	}

	installPath = filepath.Join(installDir, "dbt")

	// Make executable
	chmodErr := os.Chmod(sourcePath, 0755)
	if chmodErr != nil {
		err = fmt.Errorf("failed to make executable: %w", chmodErr)
		return installPath, err
	}

	// Copy to install location
	sourceFile, openErr := os.Open(sourcePath)
	if openErr != nil {
		err = fmt.Errorf("failed to open source: %w", openErr)
		return installPath, err
	}
	defer sourceFile.Close()

	destFile, createErr := os.Create(installPath)
	if createErr != nil {
		err = fmt.Errorf("failed to create destination: %w", createErr)
		return installPath, err
	}
	defer destFile.Close()

	_, copyErr := io.Copy(destFile, sourceFile)
	if copyErr != nil {
		err = fmt.Errorf("failed to copy binary: %w", copyErr)
		return installPath, err
	}

	// Make installed binary executable
	_ = os.Chmod(installPath, 0755)

	return installPath, err
}

// configure creates or updates the dbt configuration file.
func (i *Installer) configure(username string) (err error) {
	configPath, pathErr := GetConfigPath()
	if pathErr != nil {
		err = pathErr
		return err
	}

	// Load existing config or create new
	config, exists, loadErr := LoadExistingConfig(configPath)
	if loadErr != nil {
		err = loadErr
		return err
	}

	if !exists {
		config = &DbtConfig{
			Servers:       make(map[string]ServerConfig),
			DefaultServer: i.config.ServerName,
		}
	}

	// Build server config
	serverConfig := i.config.BuildServerConfig(username)

	// For S3, adjust the repository path (no /dbt prefix)
	if i.isS3 {
		serverConfig.Repository = i.config.ServerURL
		serverConfig.Truststore = i.config.ServerURL + "/truststore"
	}

	// Add/update server
	config.Servers[i.config.ServerName] = serverConfig

	// Set as default if this is the first server
	if config.DefaultServer == "" {
		config.DefaultServer = i.config.ServerName
	}

	// Save config
	saveErr := SaveConfig(configPath, config)
	if saveErr != nil {
		err = saveErr
		return err
	}

	fmt.Printf("  Config: %s\n", configPath)

	return err
}

// updateShellProfile adds the install directory to the user's PATH.
func (i *Installer) updateShellProfile(installPath string) (updated bool) {
	installDir := filepath.Dir(installPath)

	// Determine profile file
	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return updated
	}

	var profilePath string
	shell := os.Getenv("SHELL")

	if strings.Contains(shell, "zsh") {
		profilePath = filepath.Join(homeDir, ".zshrc")
	} else {
		profilePath = filepath.Join(homeDir, ".bashrc")
		_, statErr := os.Stat(profilePath)
		if os.IsNotExist(statErr) {
			profilePath = filepath.Join(homeDir, ".bash_profile")
		}
	}

	// macOS defaults to zsh
	if runtime.GOOS == "darwin" {
		profilePath = filepath.Join(homeDir, ".zshrc")
	}

	// Check if already configured
	content, readErr := os.ReadFile(profilePath)
	if readErr == nil {
		if strings.Contains(string(content), "# dbt PATH") ||
			strings.Contains(string(content), installDir) {
			return updated // Already configured
		}
	}

	// Append PATH configuration
	file, openErr := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		return updated
	}
	defer file.Close()

	pathLine := fmt.Sprintf("\n# dbt PATH\nexport PATH=\"%s:$PATH\"\n", installDir)
	_, writeErr := file.WriteString(pathLine)
	if writeErr != nil {
		return updated
	}

	fmt.Printf("  Added to PATH in %s\n", profilePath)
	updated = true
	return updated
}

// isInPath checks if a directory is in the current PATH.
func isInPath(dir string) (inPath bool) {
	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))

	for _, p := range paths {
		if p == dir {
			inPath = true
			return inPath
		}
	}

	return inPath
}
