//nolint:govet,usetesting // test file - shadows and t.Setenv acceptable in complex test setup
package dbt

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/nikogura/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
)

// testKeyPair holds a generated SSH key pair.
type testKeyPair struct {
	publicKey  string
	privateKey string
	keyPath    string
}

// generateTestKeyPair creates a fresh SSH key pair for testing.
func generateTestKeyPair(t *testing.T, tmpDir string) (keyPair *testKeyPair) {
	keyPath := fmt.Sprintf("%s/test_key_%d", tmpDir, time.Now().UnixNano())
	pubKeyPath := keyPath + ".pub"

	// Generate SSH key pair
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "3072", "-f", keyPath, "-N", "", "-C", "dbttester@infradel.org")
	keygenErr := cmd.Run()
	if keygenErr != nil {
		t.Fatalf("Failed to generate SSH key: %s", keygenErr)
	}

	// Read public key
	pubKeyBytes, readPubErr := os.ReadFile(pubKeyPath)
	if readPubErr != nil {
		t.Fatalf("Failed to read generated public key: %s", readPubErr)
	}
	publicKey := strings.TrimSpace(string(pubKeyBytes))

	// Read private key
	privKeyBytes, readPrivErr := os.ReadFile(keyPath)
	if readPrivErr != nil {
		t.Fatalf("Failed to read generated private key: %s", readPrivErr)
	}
	privateKey := string(privKeyBytes)

	keyPair = &testKeyPair{
		publicKey:  publicKey,
		privateKey: privateKey,
		keyPath:    keyPath,
	}
	return keyPair
}

//nolint:gocognit,gocyclo,cyclop // complex test function with multiple auth scenarios
func TestRepoServerAuth(t *testing.T) {
	type testfile struct {
		name     string
		contents string
		result   int
		auth     bool
	}

	type authinfo struct {
		user       string
		credential string
	}

	cases := []struct {
		Name           string
		ConfigTemplate string
		AuthTypeGet    string
		AuthFile       string
		AuthGet        authinfo
		TestFiles      []testfile
		AuthTypePut    string
		AuthPut        authinfo
	}{
		{
			"basic",
			`{
	"address": "127.0.0.1",
  "port": {{.Port}},
  "serverRoot": "{{.ServerRoot}}",
  "authTypeGet": "basic-htpasswd",
  "authGets": true,
  "authOptsGet": {
    "idpFile": "{{.IdpFile}}"
  },
  "authOptsPut": {
    "idpFile": "{{.IdpFile}}"
  },
	"authTypePut": "basic-htpasswd"
}`,
			"basic",
			"nik:$apr1$ytmDEY.X$LJt5T3fWtswK3KF5iINxT1",
			authinfo{
				user:       "nik",
				credential: "letmein",
			},
			[]testfile{
				{
					name:     "foo",
					contents: "frobnitz ene woo",
					result:   401,
					auth:     false,
				},
				{
					name:     "bar",
					contents: "frobnitz ene woo",
					result:   201,
					auth:     true,
				},
			},
			"basic",
			authinfo{
				user:       "nik",
				credential: "letmein",
			},
		},
		{
			"pubkey-file",
			`{
	"address": "127.0.0.1",
 "port": {{.Port}},
 "serverRoot": "{{.ServerRoot}}",
 "authTypeGet": "ssh-agent-file",
 "authGets": true,
 "authOptsGet": {
   "idpFile": "{{.IdpFile}}"
 },
 "authOptsPut": {
   "idpFile": "{{.IdpFile}}"
 },
	"authTypePut": "ssh-agent-file"
}`,
			"pubkey",
			`{
  "getUsers": [
    {
      "username": "nik",
      "publickey": "{{GENERATED_PUBLIC_KEY}}"
    }
  ],
  "putUsers": [
    {
      "username": "nik",
      "publickey": "{{GENERATED_PUBLIC_KEY}}"
    }
  ]
}
`,
			authinfo{
				user:       "nik",
				credential: "{{GENERATED_PUBLIC_KEY}}",
			},
			[]testfile{
				{
					name:     "foo",
					contents: "frobnitz ene woo",
					result:   401,
					auth:     false,
				},
				{
					name:     "bar",
					contents: "frobnitz ene woo",
					result:   201,
					auth:     true,
				},
			},
			"pubkey",
			authinfo{
				user:       "nik",
				credential: "{{GENERATED_PUBLIC_KEY}}",
			},
		},
		{
			"pubkey-func",
			`{
	"address": "127.0.0.1",
 "port": {{.Port}},
 "serverRoot": "{{.ServerRoot}}",
 "authTypeGet": "ssh-agent-func",
 "authGets": true,
 "authOptsGet": {
   "idpFunc": "echo 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org'"
 },
 "authOptsPut": {
   "idpFunc": "echo 'ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org'"
 },
	"authTypePut": "ssh-agent-func"
}`,
			"pubkey",
			`{
  "getUsers": [
    {
      "username": "nik",
      "publickey": "{{GENERATED_PUBLIC_KEY}}"
    }
  ],
  "putUsers": [
    {
      "username": "nik",
      "publickey": "{{GENERATED_PUBLIC_KEY}}"
    }
  ]
}
`,
			authinfo{
				user:       "nik",
				credential: "{{GENERATED_PUBLIC_KEY}}",
			},
			[]testfile{
				{
					name:     "foo",
					contents: "frobnitz ene woo",
					result:   401,
					auth:     false,
				},
				{
					name:     "bar",
					contents: "frobnitz ene woo",
					result:   201,
					auth:     true,
				},
			},
			"pubkey",
			authinfo{
				user:       "nik",
				credential: "{{GENERATED_PUBLIC_KEY}}",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			fmt.Printf("Testing %s auth\n", tc.Name)
			// setup temp dir
			tmpDir, err := os.MkdirTemp("", "dbt")
			if err != nil {
				t.Fatalf("Error creating temp dir %q: %s\n", tmpDir, err)
			}

			// Ensure cleanup even if test fails or times out
			t.Cleanup(func() {
				_, statErr := os.Stat(tmpDir)
				if !os.IsNotExist(statErr) {
					// Fix permissions using bulk chmod commands for better performance on large directories
					// Make all directories writable and executable by owner
					chmodDirCmd := exec.Command("find", tmpDir, "-type", "d", "-exec", "chmod", "u+rwx", "{}", "+")
					_ = chmodDirCmd.Run() // Ignore errors, try to continue

					// Make all files readable and writable by owner (especially for Go module cache read-only files)
					chmodFileCmd := exec.Command("find", tmpDir, "-type", "f", "-exec", "chmod", "u+rw", "{}", "+")
					_ = chmodFileCmd.Run() // Ignore errors, try to continue
					_ = os.RemoveAll(tmpDir)
				}
			})

			// get free port
			port, err := freeport.GetFreePort()
			if err != nil {
				t.Fatalf("Error getting a free port: %s", err)
			}

			repoDir := fmt.Sprintf("%s/repo", tmpDir)
			idpFile := fmt.Sprintf("%s/identity", tmpDir)
			configFile := fmt.Sprintf("%s/config", tmpDir)

			err = os.Mkdir(repoDir, 0755)
			if err != nil {
				t.Fatalf("Failed creating dir %q: %s", repoDir, err)
			}

			// Handle pubkey authentication tests with fresh key generation
			var oldSSHAuthSock string
			var agentPID string
			var testKeys *testKeyPair

			//nolint:nestif // pubkey test setup requires complex SSH agent initialization
			if strings.HasPrefix(tc.Name, "pubkey") {
				// Save current SSH_AUTH_SOCK
				oldSSHAuthSock = os.Getenv("SSH_AUTH_SOCK")

				// Generate fresh SSH key pair
				testKeys = generateTestKeyPair(t, tmpDir)

				// Start SSH agent
				agentCmd := exec.Command("ssh-agent", "-s")
				agentOut, err := agentCmd.Output()
				if err != nil {
					t.Fatalf("Failed to start SSH agent: %s", err)
				}

				// Parse agent output to get SSH_AUTH_SOCK and SSH_AGENT_PID
				agentEnv := string(agentOut)
				for _, line := range strings.Split(agentEnv, "\n") {
					if sockPath, found := strings.CutPrefix(line, "SSH_AUTH_SOCK="); found {
						sockPath = strings.TrimSuffix(sockPath, "; export SSH_AUTH_SOCK;")
						os.Setenv("SSH_AUTH_SOCK", sockPath)
					} else if pidStr, found := strings.CutPrefix(line, "SSH_AGENT_PID="); found {
						pidStr = strings.TrimSuffix(pidStr, "; export SSH_AGENT_PID;")
						agentPID = pidStr
						os.Setenv("SSH_AGENT_PID", pidStr)
					}
				}

				// Add key to agent
				addKeyCmd := exec.Command("ssh-add", testKeys.keyPath)
				err = addKeyCmd.Run()
				if err != nil {
					t.Fatalf("Failed to add key to SSH agent: %s", err)
				}

				// Update test case credentials to use generated key
				tc.AuthPut.credential = testKeys.publicKey
				tc.AuthGet.credential = testKeys.publicKey

				// Update config template for pubkey-func tests (replace idpFunc commands with simple echo)
				if strings.Contains(tc.Name, "func") {
					newEchoCmd := "echo '" + testKeys.publicKey + "'"

					// Replace any idpFunc value with the new echo command
					re := regexp.MustCompile(`"idpFunc":\s*"[^"]*"`)
					tc.ConfigTemplate = re.ReplaceAllString(tc.ConfigTemplate, `"idpFunc": "`+newEchoCmd+`"`)
				}

				// Replace placeholders in auth file
				updatedAuthFile := strings.ReplaceAll(tc.AuthFile, "{{GENERATED_PUBLIC_KEY}}", testKeys.publicKey)

				// Write updated auth file
				err = os.WriteFile(idpFile, []byte(updatedAuthFile), 0644)
				if err != nil {
					t.Fatalf("Failed creating auth file with generated key %s: %s", idpFile, err)
				}

				// Cleanup function to kill SSH agent and restore environment
				cleanupSSH := func() {
					// Kill the SSH agent process if we started one
					if agentPID != "" {
						killCmd := exec.Command("kill", agentPID)
						_ = killCmd.Run() // Ignore errors as process might already be dead
					}

					// Restore original SSH_AUTH_SOCK
					if oldSSHAuthSock != "" {
						os.Setenv("SSH_AUTH_SOCK", oldSSHAuthSock)
					} else {
						os.Unsetenv("SSH_AUTH_SOCK")
					}
					os.Unsetenv("SSH_AGENT_PID")

					// Clean up key files
					if testKeys != nil {
						_ = os.Remove(testKeys.keyPath)
						_ = os.Remove(testKeys.keyPath + ".pub")
					}
				}

				// Register cleanup with both defer (for normal completion) and t.Cleanup (for timeouts/panics)
				defer cleanupSSH()
				t.Cleanup(cleanupSSH)
			} else {
				// For non-pubkey tests, write auth file as is
				err = os.WriteFile(idpFile, []byte(tc.AuthFile), 0644)
				if err != nil {
					t.Fatalf("Failed creating get auth file %s: %s", idpFile, err)
				}
			}

			// write config file
			tmplData := struct {
				Port       int
				IdpFile    string
				ServerRoot string
			}{
				Port:       port,
				IdpFile:    idpFile,
				ServerRoot: repoDir,
			}

			tmpl, err := template.New(tc.Name).Parse(tc.ConfigTemplate)
			if err != nil {
				t.Fatalf("failed to parse config template for %s", tc.Name)
			}

			buf := new(bytes.Buffer)

			err = tmpl.Execute(buf, tmplData)
			if err != nil {
				t.Fatalf("Failed to execute template for %s: %s", tc.Name, err)
			}

			err = os.WriteFile(configFile, buf.Bytes(), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file %s: %s", configFile, err)
			}

			// Create reposerver
			server, err := NewRepoServer(configFile)
			if err != nil {
				t.Fatalf("Failed creating reposerver: %s\nConfig: %s\n", err, buf.String())
			}

			// start reposerver
			go server.RunRepoServer()

			testHost := fmt.Sprintf("http://%s", net.JoinHostPort(server.Address, strconv.Itoa(server.Port)))
			fmt.Printf("--- Serving requests on %s ---\n", testHost)

			fmt.Printf("Sleeping for 1 second for the test artifact server to start up.")
			time.Sleep(time.Second * 1)

			// PUT test files.  This is a basic HTTP request, not doing anything fancy through the DBT client.
			// DBT is only a reader.  How you write the files is up to you, but the auth mechanism is the same regardless.
			client := &http.Client{}

			for _, file := range tc.TestFiles {
				fileURL := fmt.Sprintf("%s/%s", testHost, file.name)

				req, err := http.NewRequest(http.MethodPut, fileURL, bytes.NewReader([]byte(file.contents)))
				if err != nil {
					t.Errorf("Failed creating request for %s: %s", file.name, err)
				}

				fmt.Printf("Writing %s to server\n", fileURL)

				if file.auth {
					switch tc.AuthTypePut {
					case "basic":
						fmt.Printf("Basic Authed Request.\n")
						req.SetBasicAuth(tc.AuthPut.user, tc.AuthPut.credential)

					case "pubkey":
						fmt.Printf("Pubkey Authed Request.\n")
						// use username and pubkey to set Token header
						// Note: parameter order may be different in upgraded jwt-ssh-agent-go
						token, err := agentjwt.SignedJwtToken(tc.AuthPut.user, "127.0.0.1", tc.AuthPut.credential)
						if err != nil {
							t.Errorf("failed to sign JWT token: %s", err)
						}

						fmt.Printf("Token in client: %q\n", token)

						if token != "" {
							fmt.Printf("Adding token header to request.\n")
							req.Header.Add("Token", token)
						}
					}
				} else {
					fmt.Printf("Unauthenticated Request.\n")
				}

				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("failed writing file %s: %s", file.name, err)
				}

				fmt.Printf("  Response: %d\n", resp.StatusCode)

				assert.Equal(t, file.result, resp.StatusCode, "File put request response code did not meet expectations.")

				// GET Test files
				// don't bother with unauthenticated files.  We don't allow that.
				//nolint:nestif // GET verification requires multiple auth scenarios
				if file.auth {
					fmt.Printf("Verifying %s exists on server\n", fileURL)

					req, err := http.NewRequest(http.MethodGet, fileURL, nil)
					if err != nil {
						t.Errorf("Failed creatign request for %s: %s", file.name, err)
					}

					md5sum, sha1sum, sha256sum, err := gomason.AllChecksumsForBytes([]byte(file.contents))
					if err != nil {
						t.Errorf("Failed checksumming %s: %s", file.name, err)
					}

					req.Header.Set("X-Checksum-Md5", md5sum)
					req.Header.Set("X-Checksum-Sha1", sha1sum)
					req.Header.Set("X-Checksum-Sha256", sha256sum)

					if file.auth {
						switch tc.AuthTypePut {
						case "basic":
							fmt.Printf("Basic Authed Request.\n")
							req.SetBasicAuth(tc.AuthGet.user, tc.AuthGet.credential)

						case "pubkey":
							fmt.Printf("Pubkey Authed Request.\n")
							// use username and pubkey to set Token header
							// Note: parameter order may be different in upgraded jwt-ssh-agent-go
							token, err := agentjwt.SignedJwtToken(tc.AuthGet.user, "127.0.0.1", tc.AuthGet.credential)
							if err != nil {
								t.Errorf("failed to sign JWT token: %s", err)
							}

							fmt.Printf("Token in client: %q\n", token)

							if token != "" {
								fmt.Printf("Adding token header to request.\n")
								req.Header.Add("Token", token)
							}
						}
					} else {
						fmt.Printf("Unauthenticated Request.\n")
					}

					resp, err := client.Do(req)
					if err != nil {
						t.Errorf("failed writing file %s: %s", file.name, err)
					}

					// verify file was put
					assert.Equal(t, 200, resp.StatusCode, "Failed to fetch file.")

					// verify file contents
					defer resp.Body.Close()

					fileBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						t.Errorf("Failed reading file contents: %s", err)
					}

					expected := file.contents
					actual := string(fileBytes)

					assert.Equal(t, expected, actual, "retrieved file contents do not meet expectations.")
				}
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	server := &DBTRepoServer{
		Address:    "127.0.0.1",
		Port:       8080,
		ServerRoot: "/tmp",
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	server.HealthHandler(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health check should return 200 OK")

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("Failed to read response body: %s", readErr)
	}
	assert.Equal(t, "ok", string(body), "Health check should return 'ok'")
}
