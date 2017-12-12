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
	http.HandleFunc("/dbt-tools/", tr.HandlerTools)
	err = http.ListenAndServe(fmt.Sprintf("localhost:%s", strconv.Itoa(port)), nil)

	return err
}

// HandlerDbt handles requests on the dbt repo path
func (tr *TestRepo) HandlerDbt(w http.ResponseWriter, r *http.Request) {
	log.Printf("*TestRepo: Request for %s*", r.URL.Path)

	if r.URL.Path == "/dbt/truststore" {
		_, err := w.Write([]byte(testKeyPublic()))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("500 - %s", err)))
		}
	}
}

// HandlerTools handles requests for tools
func (tr *TestRepo) HandlerTools(w http.ResponseWriter, r *http.Request) {

}
