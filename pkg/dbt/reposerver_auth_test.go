package dbt

import (
	"bytes"
	"fmt"
	"github.com/nikogura/gomason/pkg/gomason"
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
		AuthFile       string
		Auth           authinfo
		TestFiles      []testfile
	}{
		{
			"basic",
			`{
	"address": "127.0.0.1",
  "port": {{.Port}},
  "serverRoot": "{{.ServerRoot}}",
  "authType": "basic-htpasswd",
  "authGets": true,
  "authOpts": {
    "idpFile": "{{.IdpFile}}"
  }
}`,
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
					name:     "foo",
					contents: "frobnitz ene woo",
					result:   201,
					auth:     true,
				},
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
				t.Fatalf("Failed creating auth file %s: %s", idpFile, err)
			}

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
				t.Fatalf("Failed to execute template for %s", tc.Name)
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

			// write some files
			client := &http.Client{}

			for _, file := range tc.TestFiles {
				fileUrl := fmt.Sprintf("%s/%s", testHost, file.name)

				req, err := http.NewRequest(http.MethodPut, fileUrl, bytes.NewReader([]byte(file.contents)))
				if err != nil {
					t.Errorf("Failed creatign request for %s: %s", file.name, err)
				}

				if file.auth {
					req.SetBasicAuth(tc.Auth.user, tc.Auth.credential)
				}

				// TODO add switch for pubkey auth

				fmt.Printf("Writing %s to server\n", fileUrl)

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

					req.SetBasicAuth(tc.Auth.user, tc.Auth.credential)

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
