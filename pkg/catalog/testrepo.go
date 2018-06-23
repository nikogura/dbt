package catalog

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

	http.HandleFunc("/dbt-tools/foo/1.2.2/", tr.handlerFooVersionA)

	http.HandleFunc("/dbt-tools/foo/1.2.3/", tr.handlerFooVersionB)

	http.HandleFunc("/dbt-tools/bar/1.1.1/", tr.handlerBar)

	http.HandleFunc("/dbt-tools/foo/", tr.handlerIndex)

	http.HandleFunc("/dbt-tools/bar/", tr.handlerIndex)

	http.HandleFunc("/dbt-tools/", tr.handlerIndex)

	err = http.ListenAndServe(fmt.Sprintf("localhost:%s", strconv.Itoa(port)), nil)

	return err
}

func (tr *TestRepo) handlerIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

	switch r.URL.Path {
	// The index
	case "/dbt-tools/":
		_, err := w.Write([]byte(repoIndex()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	case "/dbt-tools/foo/":
		_, err := w.Write([]byte(fooIndex()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/bar/":
		_, err := w.Write([]byte(barIndex()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (tr *TestRepo) handlerFooVersionA(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

	switch r.URL.Path {
	// Index
	case "/dbt-tools/foo/1.2.2/description.txt":
		_, err := w.Write([]byte(fooDescription()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (tr *TestRepo) handlerFooVersionB(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt-tools/foo/1.2.3/description.txt":
		_, err := w.Write([]byte(fooDescription()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (tr *TestRepo) handlerBar(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

	switch r.URL.Path {
	// Index
	case "/dbt-tools/bar/1.1.1/description.txt":
		_, err := w.Write([]byte(barDescription()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
