package dbt

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// TestRepo a fake repository server.  Basically an in-memory http server that can be used as a test fixture for testing the internal API.  Cool huh?
type TestRepo struct{}

// Run runs the test repository server.
func (tr *TestRepo) Run(port int) (err error) {

	log.Printf("Running test artifact server on port %d", port)
	http.HandleFunc("/dbt/", tr.HandlerDbt)

	// /dbt-tools/foo/ work
	// /dbt-tools/bar/ do not
	http.HandleFunc("/dbt-tools/foo/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

		switch r.URL.Path {
		// The index
		case "/dbt-tools/foo/":
			_, err := w.Write([]byte(dbtIndexOutput()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	http.HandleFunc("/dbt-tools/foo/1.2.2/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

		switch r.URL.Path {
		// Index
		case "/dbt-tools/foo/1.2.2/":
			_, err := w.Write([]byte(dbtVersionAIndexOutput()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.2/linux/x86_64/foo":
			_, err := w.Write([]byte(dbtVersionAContent()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.2/linux/x86_64/foo.md5":
			_, err := w.Write([]byte(dbtVersionAMd5()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.2/linux/x86_64/foo.sha1":
			_, err := w.Write([]byte(dbtVersionASha1()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.2/linux/x86_64/foo.sha256":
			_, err := w.Write([]byte(dbtVersionASha256()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.2/linux/x86_64/foo.asc":
			_, err := w.Write([]byte(dbtVersionASig()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	http.HandleFunc("/dbt-tools/foo/1.2.3/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

		switch r.URL.Path {
		// The index
		case "/dbt-tools/foo/1.2.3/":
			_, err := w.Write([]byte(dbtVersionBIndexOutput()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.3/linux/x86_64/foo":
			_, err := w.Write([]byte(dbtVersionBContent()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.3/linux/x86_64/foo.md5":
			_, err := w.Write([]byte(dbtVersionBMd5()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.3/linux/x86_64/foo.sha1":
			_, err := w.Write([]byte(dbtVersionBSha1()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.3/linux/x86_64/foo.sha256":
			_, err := w.Write([]byte(dbtVersionBSha256()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.3/linux/x86_64/foo.asc":
			_, err := w.Write([]byte(dbtVersionBSig()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	err = http.ListenAndServe(fmt.Sprintf("localhost:%s", strconv.Itoa(port)), nil)

	return err
}

// HandlerDbt handles requests on the dbt repo path
func (tr *TestRepo) HandlerDbt(w http.ResponseWriter, r *http.Request) {
	log.Printf("*TestRepo: DBT Request for %s*", r.URL.Path)

	if r.URL.Path == "/dbt/truststore" {
		_, err := w.Write([]byte(testKeyPublic()))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("500 - %s", err)))
		}
	}
}
