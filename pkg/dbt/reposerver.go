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

// DBTRepoServer The reference 'trusted repository' server for dbt.
type DBTRepoServer struct {
	Address    string
	Port       int
	ServerRoot string
	AuthType   string
	AuthGets   bool
	AuthOpts
}

// AuthOpts Struct for holding Auth options
type AuthOpts struct {
	IdpFile string
	IdpFunc string
}

// AUTH_BASIC_HTPASSWD config flag for basic auth
const AUTH_BASIC_HTPASSWD = "basic-htpasswd"

// AUTH_SSH_AGENT_FILE config setting for file based ssh-agent auth (file mapping principals to public keys similer to .htaccess files)
const AUTH_SSH_AGENT_FILE = "ssh-agent-file"

// AUTH_SSH_AGENT_FUNC config setting for using a shell function to retrieve the public key for a principal
const AUTH_SSH_AGENT_FUNC = "ssh-agent-func"

// AUTH_BASIC_LDAP config flag for user/password auth off an LDAP directory server
const AUTH_BASIC_LDAP = "basic-ldap"

// AUTH_SSH_AGENT_LDAP flag for configuring ssh-agent auth pulling public key from an LDAP directory
const AUTH_SSH_AGENT_LDAP = "ssh-agent-ldap"

// RunRepoServer Run runs the test repository server.
func (d *DBTRepoServer) RunRepoServer() (err error) {

	log.Printf("Running dbt artifact server on %s port %d.  Serving tree at: %s", d.Address, d.Port, d.ServerRoot)

	fullAddress := fmt.Sprintf("%s:%s", d.Address, strconv.Itoa(d.Port))

	r := mux.NewRouter()

	// handle the uploads if enabled
	//if d.AuthType != "" {
	//	switch d.AuthType {
	//	case AUTH_BASIC_HTPASSWD:
	//		htpasswd := auth.HtpasswdFileProvider(d.AuthOpts.IdpFile)
	//		authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
	//		r.PathPrefix("/").HandlerFunc(authenticator.Wrap(d.PutHandlerHtpasswd)).Methods("PUT")
	//	case AUTH_SSH_AGENT_FILE:
	//		r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFile).Methods("PUT")
	//	case AUTH_SSH_AGENT_FUNC:
	//		r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFunc).Methods("PUT")
	//	case AUTH_BASIC_LDAP:
	//		err = errors.New("basic auth via ldap not yet supported")
	//		return err
	//	case AUTH_SSH_AGENT_LDAP:
	//		err = errors.New("ssh-agent auth via ldap not yet supported")
	//		return err
	//	default:
	//		err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthType))
	//		return err
	//	}
	//}

	// handle the downloads and indices
	//if d.AuthType != "" && d.AuthGets {
	//	switch d.AuthType {
	//	case AUTH_BASIC_HTPASSWD:
	//		htpasswd := auth.HtpasswdFileProvider(d.AuthOpts.IdpFile)
	//		authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
	//		r.PathPrefix("/").Handler(auth.JustCheck(authenticator, http.FileServer(http.Dir(d.ServerRoot)).ServeHTTP)).Methods("GET", "HEAD")
	//	case AUTH_SSH_AGENT_FILE:
	//
	//	case AUTH_SSH_AGENT_FUNC:
	//
	//	case AUTH_BASIC_LDAP:
	//		err = errors.New("basic auth via ldap not yet supported")
	//		return err
	//	case AUTH_SSH_AGENT_LDAP:
	//		err = errors.New("ssh-agent auth via ldap not yet supported")
	//		return err
	//	default:
	//		err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthType))
	//		return err
	//	}
	//} else {
	r.PathPrefix("/").Handler(http.FileServer(http.Dir(d.ServerRoot))).Methods("GET", "HEAD")
	//}

	// run the server
	err = http.ListenAndServe(fullAddress, r)

	return err
}

// TODO verify sent checksums
// TODO create dirs if they don't exist
// TODO how to handle privilege separation?  Different htpasswd files?

//// HandlePut actually does the work of writing the uploaded files to the filesystem
//func (d *DBTRepoServer) HandlePut() {
//
//	// TODO handle put
//}
//
//// PutHandlerHtpasswd Handles puts with htpasswd auth
//func (d *DBTRepoServer) PutHandlerHtpasswd(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
//	log.Printf("Received Put")
//	w.WriteHeader(http.StatusBadRequest)
//
//	d.HandlePut()
//}
//
//// PubkeyFromFile takes a subject name, and pulls the corresponding pubkey out of the identity provider file
//func (d *DBTRepoServer) PubkeyFromFile(subject string) (pubkey string, err error) {
//	// need to get pubkey file similar to: htpasswd := PubkeyFileProvider(d.AuthOpts.IdpFile)
//	return pubkey, err
//}
//
//// PubkeyFromFunc takes a subject name, and runs the configured function to return the corresponding public key
//func (d *DBTRepoServer) PubkeyFromFunc(subject string) (pubkey string, err error) {
//
//	return pubkey, err
//}
//
//func (d *DBTRepoServer) PubkeyAuth(subject string, authFunc func(subject string) (pubkey string, err error)) (principal string) {
//
//	return principal
//}
//
//// PutHandlerPubKeyFile
//func (d *DBTRepoServer) PutHandlerPubkeyFile(w http.ResponseWriter, r *http.Request) {
//	tokenString := r.Header.Get("Token")
//
//	// Parse the token, which includes setting up it's internals so it can be verified.
//	subject, token, err := agentjwt.ParsePubkeySignedToken(tokenString, d.PubkeyFromFile)
//	if err != nil {
//		log.Errorf("Error: %s", err)
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	if !token.Valid {
//		log.Info("Auth Failed")
//		w.WriteHeader(http.StatusUnauthorized)
//	}
//
//	log.Infof("Subject %s successfuly authenticated", subject)
//
//	d.HandlePut()
//}
//
//// PutHandlerPubkeyFunc
//func (d *DBTRepoServer) PutHandlerPubkeyFunc(w http.ResponseWriter, r *http.Request) {
//	tokenString := r.Header.Get("Token")
//
//	// sanity check username
//
//	//Parse the token, which includes setting up it's internals so it can be verified.
//	subject, token, err := agentjwt.ParsePubkeySignedToken(tokenString, d.PubkeyFromFunc)
//	if err != nil {
//		log.Errorf("Error: %s", err)
//		w.WriteHeader(http.StatusBadRequest)
//		return
//	}
//
//	if !token.Valid {
//		log.Info("Auth Failed")
//		w.WriteHeader(http.StatusUnauthorized)
//	}
//
//	log.Infof("Subject %s successfuly authenticated", subject)
//
//	d.HandlePut()
//}

// Auth Methods
// basic-htpasswd
// ssh-agent-file
// ssh-agent-func

// basic-ldap
// pubkey-ldap

// if basic-htpasswd, need file
// if pubkey-file, need file
// if pubkey-func, need func
// if basic-ldap - need ldap creds/info
// if pubkey-ldap - need ldap creds/info

// Identity Providers
// both htpasswd and pubkey methods need a source for the hashes or public keys - the Identity Provider
// That source could be a file, a function, or LDAP.

// If it 's a file, we read the file for user and verify
// If it's a func, we call it with the username, and expect to get a 'thing' back that can be used for auth.
// Either way, we aught to be able to detect

// Auth file - expected to be either an htpasswd file, or a similar file set up for public keys
// htpasswd:
// 	 name:<hash>
// ssh-agent:
//   name:ssh-rsa asdfsdfsdfsdfsdfsdfsd comment

// read the file, and decide what type of auth it is based on contents.

// AuthFunc should expect to take a username, and return *something* that can be parsed as either an htpasswd hash or a public key.
