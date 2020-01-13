package dbt

import (
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

// DBTRepoServer
type DBTRepoServer struct {
	Address    string
	Port       int
	ServerRoot string
	PubkeyFunc func(username string) (pubkey string, err error)
}

// Run runs the test repository server.
func (d *DBTRepoServer) RunRepoServer() (err error) {

	log.Printf("Running dbt artifact server on %s port %d.  Serving tree at: %s", d.Address, d.Port, d.ServerRoot)

	fullAddress := fmt.Sprintf("%s:%s", d.Address, strconv.Itoa(d.Port))

	r := mux.NewRouter()

	// handle the uploads
	r.PathPrefix("/").Handler(http.HandlerFunc(d.PutHandler)).Methods("PUT")

	// handle the downloads and indices
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(d.ServerRoot))).Methods("GET")

	// run the server
	err = http.ListenAndServe(fullAddress, r)

	return err
}

// TODO verify sent checksums

func (d *DBTRepoServer) PutHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received Put")
	w.WriteHeader(http.StatusBadRequest)

	//tokenString := r.Header.Get("Token")

	// Parse the token, which includes setting up it's internals so it can be verified.
	//subject, token, err := ParsePubkeySignedToken(tokenString, d.PubkeyFunc)
	//if err != nil {
	//	log.Errorf("Error: %s", err)
	//	w.WriteHeader(http.StatusBadRequest)
	//	return
	//}
	//
	//if !token.Valid {
	//	log.Info("Auth Failed")
	//	w.WriteHeader(http.StatusUnauthorized)
	//}
	//
	//log.Infof("Subject %s successfuly authenticated", subject)

}
