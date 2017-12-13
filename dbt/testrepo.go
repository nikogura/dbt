package dbt

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// TestRepo a struct for a test repository server
type TestRepo struct{}

// Run runs the test repository server
func (tr *TestRepo) Run(port int) (err error) {

	log.Printf("Running test artifact server on port %d", port)
	http.HandleFunc("/dbt/", tr.HandlerDbt)

	// /dbt-tools/foo/ work
	// /dbt-tools/bar/ do not
	http.HandleFunc("/dbt-tools/foo/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

		switch r.URL.Path {
		case "/dbt-tools/foo/":
			_, err := w.Write([]byte(dbtIndexOutput()))
			if err != nil {
				log.Printf("Failed to write response: %s", err)
			}
		case "/dbt-tools/foo/1.2.2":

		case "/dbt-tools/foo/1.2.3":

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
