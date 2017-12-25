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

	http.HandleFunc("/dbt/truststore", tr.HandlerTruststore)

	http.HandleFunc("/dbt-tools/foo/1.2.2/", tr.HandlerVersionA)

	http.HandleFunc("/dbt-tools/foo/1.2.3/", tr.HandlerVersionB)

	http.HandleFunc("/dbt-tools/foo/", tr.HandlerTool)

	http.HandleFunc("/dbt/1.2.2/darwin/", tr.DbtHandlerVersionADarwin)

	http.HandleFunc("/dbt/1.2.3/darwin/", tr.DbtHandlerVersionBDarwin)

	http.HandleFunc("/dbt/1.2.2/linux/", tr.DbtHandlerVersionALinux)

	http.HandleFunc("/dbt/1.2.3/linux/", tr.DbtHandlerVersionBLinux)

	http.HandleFunc("/dbt/", tr.HandlerDbt)

	err = http.ListenAndServe(fmt.Sprintf("localhost:%s", strconv.Itoa(port)), nil)

	return err
}

// HandlerTruststore handles requests on the dbt repo path
func (tr *TestRepo) HandlerTruststore(w http.ResponseWriter, r *http.Request) {
	log.Printf("*TestRepo: DBT Request for %s*", r.URL.Path)

	_, err := w.Write([]byte(testTruststore()))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("500 - %s", err)))
	}
}

// HandlerTool handles requests for the 'foo' tool
func (tr *TestRepo) HandlerTool(w http.ResponseWriter, r *http.Request) {
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
}

// HandlerDbt handles requests for the 'foo' tool
func (tr *TestRepo) HandlerDbt(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Root handler dbt Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt/":
		_, err := w.Write([]byte(dbtIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// HandlerVersionA handles requests for version A
func (tr *TestRepo) HandlerVersionA(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

	switch r.URL.Path {
	// Index
	case "/dbt-tools/foo/1.2.2/":
		_, err := w.Write([]byte(dbtVersionAIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.2/linux/amd64/foo":
		_, err := w.Write([]byte(dbtVersionAContent()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.2/linux/amd64/foo.md5":
		_, err := w.Write([]byte(dbtVersionAMd5()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.2/linux/amd64/foo.sha1":
		_, err := w.Write([]byte(dbtVersionASha1()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.2/linux/amd64/foo.sha256":
		_, err := w.Write([]byte(dbtVersionASha256()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.2/linux/amd64/foo.asc":
		_, err := w.Write([]byte(dbtVersionASig()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// HandlerVersionB  handles requests for version B
func (tr *TestRepo) HandlerVersionB(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: Tool Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt-tools/foo/1.2.3/":
		_, err := w.Write([]byte(dbtVersionBIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.3/linux/amd64/foo":
		_, err := w.Write([]byte(dbtVersionBContent()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.3/linux/amd64/foo.md5":
		_, err := w.Write([]byte(dbtVersionBMd5()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.3/linux/amd64/foo.sha1":
		_, err := w.Write([]byte(dbtVersionBSha1()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.3/linux/amd64/foo.sha256":
		_, err := w.Write([]byte(dbtVersionBSha256()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt-tools/foo/1.2.3/linux/amd64/foo.asc":
		_, err := w.Write([]byte(dbtVersionBSig()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// DbtHandlerVersionADarwin handles requests for version A
func (tr *TestRepo) DbtHandlerVersionADarwin(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: dbt Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt/":
		_, err := w.Write([]byte(dbtIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/darwin/amd64/dbt":
		_, err := w.Write([]byte(dbtVersionAContent()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/darwin/amd64/dbt.md5":
		_, err := w.Write([]byte(dbtVersionAMd5()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/darwin/amd64/dbt.sha1":
		_, err := w.Write([]byte(dbtVersionASha1()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/darwin/amd64/dbt.sha256":
		_, err := w.Write([]byte(dbtVersionASha256()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/darwin/amd64/dbt.asc":
		_, err := w.Write([]byte(dbtVersionASig()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// DbtHandlerVersionALinux handles requests for version A
func (tr *TestRepo) DbtHandlerVersionALinux(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: dbt Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt/":
		_, err := w.Write([]byte(dbtIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/linux/amd64/dbt":
		_, err := w.Write([]byte(dbtVersionAContent()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/linux/amd64/dbt.md5":
		_, err := w.Write([]byte(dbtVersionAMd5()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/linux/amd64/dbt.sha1":
		_, err := w.Write([]byte(dbtVersionASha1()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/linux/amd64/dbt.sha256":
		_, err := w.Write([]byte(dbtVersionASha256()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.2/linux/amd64/dbt.asc":
		_, err := w.Write([]byte(dbtVersionASig()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// DbtHandlerVersionBDarwin  handles requests for version B
func (tr *TestRepo) DbtHandlerVersionBDarwin(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: dbt Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt/":
		_, err := w.Write([]byte(dbtIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/darwin/amd64/dbt":
		_, err := w.Write([]byte(dbtVersionBContent()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/darwin/amd64/dbt.md5":
		_, err := w.Write([]byte(dbtVersionBMd5()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/darwin/amd64/dbt.sha1":
		_, err := w.Write([]byte(dbtVersionBSha1()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/darwin/amd64/dbt.sha256":
		_, err := w.Write([]byte(dbtVersionBSha256()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/darwin/amd64/dbt.asc":
		_, err := w.Write([]byte(dbtVersionBSig()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// DbtHandlerVersionBLinux  handles requests for version B
func (tr *TestRepo) DbtHandlerVersionBLinux(w http.ResponseWriter, r *http.Request) {
	log.Printf("**TestRepo: dbt Request for %s", r.URL.Path)

	switch r.URL.Path {
	case "/dbt/":
		_, err := w.Write([]byte(dbtIndexOutput()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/linux/amd64/dbt":
		_, err := w.Write([]byte(dbtVersionBContent()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/linux/amd64/dbt.md5":
		_, err := w.Write([]byte(dbtVersionBMd5()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/linux/amd64/dbt.sha1":
		_, err := w.Write([]byte(dbtVersionBSha1()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/linux/amd64/dbt.sha256":
		_, err := w.Write([]byte(dbtVersionBSha256()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}
	case "/dbt/1.2.3/linux/amd64/dbt.asc":
		_, err := w.Write([]byte(dbtVersionBSig()))
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

	default:
		w.WriteHeader(http.StatusNotFound)
	}
}
