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
  "authOpts": {
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
			"pubkey",
			`{
	"address": "127.0.0.1",
 "port": {{.Port}},
 "serverRoot": "{{.ServerRoot}}",
 "authTypeGet": "ssh-agent-file",
 "authGets": true,
 "authOpts": {
   "idpFile": "{{.IdpFile}}"
 },
	"authTypePut": "ssh-agent-file"
}`,
			"pubkey",
			`{
  "getUsers": [
    {
      "username": "nik",
      "keys": [
        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQD3ETSz3t6zQCMaf+mAY9/gGc/e3Yuj5miepHfneRIqXx+jvULL/h6ldNjH4wvtf9GCr/KwJUVxL+k1UqzPwSYLnEXfsO3qLD6JmBSMtyiuOrfIBJqU7NimtSm8fPL9wF/J5ACrl99T5qR7+ykks7W8cbbZ/UY+PPO0SN0E7LYvKWBGnl7M0ah0Hyofg7xiIhrTQf+CBXj7mM1vBi+HbTnRR+Nl5+X9d78y4j1aI9LvjeOOPU1sVEDdcYWsu4xFqXl12hnjSHRcLNlTThO4T0k+EPVQ4ryBM5HC14lkDIacCaR4Dtfz909NvVGR+4Y5aE1OxevzAxxJSrSdpkTbKKDA8qTaMO56gjQ1saS7i7Bv5SzReGaUqv0sZi0xDhYFz4lEvfsa82q7ic9s10kkrrYQYUF8lFS1lBnAthN6Mu/10Iorf/KvG84OltYVAAhotCnER9dkvTEU7eyIX4ITfOO50cbvzwZk5sk/vATvuYfO7+V9w7N8P5z2pOipbFtCsW6aV1cH+frKA7MR0aZRSfdzPjINvp25HxT/ctZZxwacrzpY3GSofh2hMNfitUjZvHyLRcRY0Zx0iGON3531RTwR9j8+95HhWvuYGFC41sUfWdUjHWpnNagX5PS7JlFvT4ha0LOnElpfTgiVVcCfLdURByK1stwhT0H2Z4lGrqvjEQ== vayde@Talathar.local"
      ]
    }
  ],
  "putUsers": [
    {
      "username": "nik",
      "keys": [
        "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQD3ETSz3t6zQCMaf+mAY9/gGc/e3Yuj5miepHfneRIqXx+jvULL/h6ldNjH4wvtf9GCr/KwJUVxL+k1UqzPwSYLnEXfsO3qLD6JmBSMtyiuOrfIBJqU7NimtSm8fPL9wF/J5ACrl99T5qR7+ykks7W8cbbZ/UY+PPO0SN0E7LYvKWBGnl7M0ah0Hyofg7xiIhrTQf+CBXj7mM1vBi+HbTnRR+Nl5+X9d78y4j1aI9LvjeOOPU1sVEDdcYWsu4xFqXl12hnjSHRcLNlTThO4T0k+EPVQ4ryBM5HC14lkDIacCaR4Dtfz909NvVGR+4Y5aE1OxevzAxxJSrSdpkTbKKDA8qTaMO56gjQ1saS7i7Bv5SzReGaUqv0sZi0xDhYFz4lEvfsa82q7ic9s10kkrrYQYUF8lFS1lBnAthN6Mu/10Iorf/KvG84OltYVAAhotCnER9dkvTEU7eyIX4ITfOO50cbvzwZk5sk/vATvuYfO7+V9w7N8P5z2pOipbFtCsW6aV1cH+frKA7MR0aZRSfdzPjINvp25HxT/ctZZxwacrzpY3GSofh2hMNfitUjZvHyLRcRY0Zx0iGON3531RTwR9j8+95HhWvuYGFC41sUfWdUjHWpnNagX5PS7JlFvT4ha0LOnElpfTgiVVcCfLdURByK1stwhT0H2Z4lGrqvjEQ== vayde@Talathar.local"
      ]
    }
  ]
}
`,
			authinfo{
				user:       "nik",
				credential: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQD3ETSz3t6zQCMaf+mAY9/gGc/e3Yuj5miepHfneRIqXx+jvULL/h6ldNjH4wvtf9GCr/KwJUVxL+k1UqzPwSYLnEXfsO3qLD6JmBSMtyiuOrfIBJqU7NimtSm8fPL9wF/J5ACrl99T5qR7+ykks7W8cbbZ/UY+PPO0SN0E7LYvKWBGnl7M0ah0Hyofg7xiIhrTQf+CBXj7mM1vBi+HbTnRR+Nl5+X9d78y4j1aI9LvjeOOPU1sVEDdcYWsu4xFqXl12hnjSHRcLNlTThO4T0k+EPVQ4ryBM5HC14lkDIacCaR4Dtfz909NvVGR+4Y5aE1OxevzAxxJSrSdpkTbKKDA8qTaMO56gjQ1saS7i7Bv5SzReGaUqv0sZi0xDhYFz4lEvfsa82q7ic9s10kkrrYQYUF8lFS1lBnAthN6Mu/10Iorf/KvG84OltYVAAhotCnER9dkvTEU7eyIX4ITfOO50cbvzwZk5sk/vATvuYfO7+V9w7N8P5z2pOipbFtCsW6aV1cH+frKA7MR0aZRSfdzPjINvp25HxT/ctZZxwacrzpY3GSofh2hMNfitUjZvHyLRcRY0Zx0iGON3531RTwR9j8+95HhWvuYGFC41sUfWdUjHWpnNagX5PS7JlFvT4ha0LOnElpfTgiVVcCfLdURByK1stwhT0H2Z4lGrqvjEQ== vayde@Talathar.local",
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
				credential: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQD3ETSz3t6zQCMaf+mAY9/gGc/e3Yuj5miepHfneRIqXx+jvULL/h6ldNjH4wvtf9GCr/KwJUVxL+k1UqzPwSYLnEXfsO3qLD6JmBSMtyiuOrfIBJqU7NimtSm8fPL9wF/J5ACrl99T5qR7+ykks7W8cbbZ/UY+PPO0SN0E7LYvKWBGnl7M0ah0Hyofg7xiIhrTQf+CBXj7mM1vBi+HbTnRR+Nl5+X9d78y4j1aI9LvjeOOPU1sVEDdcYWsu4xFqXl12hnjSHRcLNlTThO4T0k+EPVQ4ryBM5HC14lkDIacCaR4Dtfz909NvVGR+4Y5aE1OxevzAxxJSrSdpkTbKKDA8qTaMO56gjQ1saS7i7Bv5SzReGaUqv0sZi0xDhYFz4lEvfsa82q7ic9s10kkrrYQYUF8lFS1lBnAthN6Mu/10Iorf/KvG84OltYVAAhotCnER9dkvTEU7eyIX4ITfOO50cbvzwZk5sk/vATvuYfO7+V9w7N8P5z2pOipbFtCsW6aV1cH+frKA7MR0aZRSfdzPjINvp25HxT/ctZZxwacrzpY3GSofh2hMNfitUjZvHyLRcRY0Zx0iGON3531RTwR9j8+95HhWvuYGFC41sUfWdUjHWpnNagX5PS7JlFvT4ha0LOnElpfTgiVVcCfLdURByK1stwhT0H2Z4lGrqvjEQ== vayde@Talathar.local",
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

			err = ioutil.WriteFile(idpFile, []byte(tc.AuthFile), 0644)
			if err != nil {
				t.Fatalf("Failed creating get auth file %s: %s", idpFile, err)
			}

			// TODO make test case work off of test key, not my own
			// TODO start ssh agent if necessary

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

			err = ioutil.WriteFile(configFile, buf.Bytes(), 0644)
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

			// Write test files.  This is a basic HTTP request, not doing anything fancy through the DBT client.
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

				// verify file was written
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
