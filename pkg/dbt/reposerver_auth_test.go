package dbt

import (
	"bytes"
	"fmt"
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/orion-labs/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
)

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
      "publickey": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org"
    }
  ],
  "putUsers": [
    {
      "username": "nik",
      "publickey": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org"
    }
  ]
}
`,
			authinfo{
				user:       "nik",
				credential: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org",
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
				credential: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org",
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
      "publickey": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org"
    }
  ],
  "putUsers": [
    {
      "username": "nik",
      "publickey": "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org"
    }
  ]
}
`,
			authinfo{
				user:       "nik",
				credential: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org",
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
				credential: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC6jJu0QdJfhaa8d1EH33/ee8p1JgS885g8P+s4DWbuCYdYITcuHtRq+DqgEeZGBGtocQcv2CFpzHS2K3JZzB8000tz/SOgZHT1ywqCBkaA0HObBR2cpgkC2qUmQT0WFz6/+yOF22KAqKoIRNucwTKPgQGpYeWD13ALMEvh7q1Z1HmIMKdeMCo6ziBkPiMGAbPpKqzjpUbKXaT+PkE37ouCs3YygZv6UtcTzCEsY4CIpuB45FjLKhAhA26wPVsKBSUiJCMwLhN46jDDhJ8BFSv0nUYVBT/+4nriaMeMtKO9lZ6VzHnIYzGmSWH1OWxWdRA1AixJmk2RSWlAq9yIBRJk9Tdc457j7em0hohdCGEeGyb1VuSoiEiHScnPeWsLYjc/kJIBL40vTQRyiNCT+M+7p6BlT9aTBuXsv9Njw2K60u+ekoAOE4+wlKKYNrEj09yYvdl9hVrI1bNg22JsXTYqOe4TT7Cki47cYF9QwwXPZbTBRmdDX6ftOhwBzas2mAs= dbttester@infradel.org",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			fmt.Printf("Testing %s auth\n", tc.Name)
			// setup temp dir
			tmpDir, err := ioutil.TempDir("", "dbt")
			if err != nil {
				t.Fatalf("Error creating temp dir %q: %s\n", tmpDir, err)
			}

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

			err = os.WriteFile(idpFile, []byte(tc.AuthFile), 0644)
			if err != nil {
				t.Fatalf("Failed creating get auth file %s: %s", idpFile, err)
			}

			// TODO start ssh agent if necessary?

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

			testHost := fmt.Sprintf("http://%s:%d", server.Address, server.Port)
			fmt.Printf("--- Serving requests on %s ---\n", testHost)

			fmt.Printf("Sleeping for 1 second for the test artifact server to start up.")
			time.Sleep(time.Second * 1)

			// PUT test files.  This is a basic HTTP request, not doing anything fancy through the DBT client.
			// DBT is only a reader.  How you write the files is up to you, but the auth mechanism is the same regardless.
			client := &http.Client{}

			for _, file := range tc.TestFiles {
				fileUrl := fmt.Sprintf("%s/%s", testHost, file.name)

				req, err := http.NewRequest(http.MethodPut, fileUrl, bytes.NewReader([]byte(file.contents)))
				if err != nil {
					t.Errorf("Failed creating request for %s: %s", file.name, err)
				}

				fmt.Printf("Writing %s to server\n", fileUrl)

				if file.auth {
					switch tc.AuthTypePut {
					case "basic":
						fmt.Printf("Basic Authed Request.\n")
						req.SetBasicAuth(tc.AuthPut.user, tc.AuthPut.credential)

					case "pubkey":
						fmt.Printf("Pubkey Authed Request.\n")
						// use username and pubkey to set Token header
						token, err := agentjwt.SignedJwtToken(tc.AuthPut.user, tc.AuthPut.credential)
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
				if file.auth {
					fmt.Printf("Verifying %s exists on server\n", fileUrl)

					req, err := http.NewRequest(http.MethodGet, fileUrl, nil)
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
							token, err := agentjwt.SignedJwtToken(tc.AuthGet.user, tc.AuthGet.credential)
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

					fileBytes, err := ioutil.ReadAll(resp.Body)
					if err != nil {
						t.Errorf("Failed reading file contents: %s", err)
					}

					expected := file.contents
					actual := string(fileBytes)

					assert.Equal(t, expected, actual, "retrieved file contents do not meet expectations.")
				}
			}

			// cleanup
			if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
				_ = os.Remove(tmpDir)
			}
		})
	}
}
