package dbt

import (
	"encoding/json"
	"fmt"
	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/orion-labs/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

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

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

// DBTRepoServer The reference 'trusted repository' server for dbt.
type DBTRepoServer struct {
	Address     string   `json:"address"`
	Port        int      `json:"port"`
	ServerRoot  string   `json:"serverRoot"`
	AuthTypeGet string   `json:"authTypeGet"`
	AuthTypePut string   `json:"authTypePut"`
	AuthGets    bool     `json:"authGets"`
	AuthOptsGet AuthOpts `json:"authOptsGet"`
	AuthOptsPut AuthOpts `json:"authOptsPut"`
}

// AuthOpts Struct for holding Auth options
type AuthOpts struct {
	IdpFile string `json:"idpFile"`
	IdpFunc string `json:"idpFunc,omitempty"`
}

// NewRepoServer creates a new DBTRepoServer object from the config file provided.
func NewRepoServer(configFilePath string) (server *DBTRepoServer, err error) {
	c, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		err = errors.Wrapf(err, "failed to read config file %q", configFilePath)
		return server, err
	}

	server = &DBTRepoServer{}

	err = json.Unmarshal(c, server)
	if err != nil {
		err = errors.Wrapf(err, "failed to unmarshal json in %q", configFilePath)
	}

	return server, err
}

// RunRepoServer Run runs the test repository server.
func (d *DBTRepoServer) RunRepoServer() (err error) {

	log.Printf("Running dbt artifact server on %s port %d.  Serving tree at: %s", d.Address, d.Port, d.ServerRoot)

	fullAddress := fmt.Sprintf("%s:%s", d.Address, strconv.Itoa(d.Port))

	r := mux.NewRouter()

	// handle the uploads if enabled
	if d.AuthTypePut != "" {
		switch d.AuthTypePut {
		case AUTH_BASIC_HTPASSWD:
			htpasswd := auth.HtpasswdFileProvider(d.AuthOptsPut.IdpFile)
			authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
			r.PathPrefix("/").HandlerFunc(authenticator.Wrap(d.PutHandlerHtpasswd)).Methods("PUT")
		case AUTH_SSH_AGENT_FILE:
			r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFile).Methods("PUT")
		case AUTH_SSH_AGENT_FUNC:
			r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFunc).Methods("PUT")
		//case AUTH_BASIC_LDAP:
		//	err = errors.New("basic auth via ldap not yet supported")
		//	return err
		//case AUTH_SSH_AGENT_LDAP:
		//	err = errors.New("ssh-agent auth via ldap not yet supported")
		//	return err
		default:
			err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthTypePut))
			return err
		}
	}

	// handle the downloads and indices
	if d.AuthTypeGet != "" && d.AuthGets {
		switch d.AuthTypeGet {
		case AUTH_BASIC_HTPASSWD:
			htpasswd := auth.HtpasswdFileProvider(d.AuthOptsGet.IdpFile)
			authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
			r.PathPrefix("/").Handler(auth.JustCheck(authenticator, http.FileServer(http.Dir(d.ServerRoot)).ServeHTTP)).Methods("GET", "HEAD")
		case AUTH_SSH_AGENT_FILE:
			r.PathPrefix("/").Handler(d.CheckPubkeysGetFile(http.FileServer(http.Dir(d.ServerRoot)).ServeHTTP)).Methods("GET", "HEAD")

		case AUTH_SSH_AGENT_FUNC:
			r.PathPrefix("/").Handler(d.CheckPubkeysGetFunc(http.FileServer(http.Dir(d.ServerRoot)).ServeHTTP)).Methods("GET", "HEAD")

		//case AUTH_BASIC_LDAP:
		//	err = errors.New("basic auth via ldap not yet supported")
		//	return err
		//
		//case AUTH_SSH_AGENT_LDAP:
		//	err = errors.New("ssh-agent auth via ldap not yet supported")
		//	return err
		//
		default:
			err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthTypeGet))
			return err

		}
	} else {
		r.PathPrefix("/").Handler(http.FileServer(http.Dir(d.ServerRoot))).Methods("GET", "HEAD")
	}

	// run the server
	err = http.ListenAndServe(fullAddress, r)

	return err
}

func (d *DBTRepoServer) HandlePut(path string, body io.ReadCloser, md5sum string, sha1sum string, sha256sum string) (err error) {
	filePath := fmt.Sprintf("%s/%s", d.ServerRoot, path)
	fileDir := filepath.Dir(filePath)

	// create subdirs if they don't exist
	if _, err := os.Stat(fileDir); os.IsNotExist(err) {
		err = os.MkdirAll(fileDir, 0755)
		if err != nil {
			err = errors.Wrapf(err, "failed to create server path %s", fileDir)
			return err
		}
	}

	fileBytes, err := ioutil.ReadAll(body)

	// Checksum bytes
	md5Actual, sha1Actual, sha256Actual, err := gomason.AllChecksumsForBytes(fileBytes)
	if err != nil {
		err = errors.Wrapf(err, "failed to derive checksums for file %s", filePath)
		return err
	}

	// verify sent checksums if present.  You don't have to provide checksums, but if you do, they have to match what we received.
	if md5sum != "" {
		if md5sum != md5Actual {
			err = errors.New(fmt.Sprintf("Md5 sum for %s is %s which does not match the expected %s.", filePath, md5Actual, md5sum))
			return err
		}
	}

	if sha1sum != "" {
		if sha1sum != sha1Actual {
			err = errors.New(fmt.Sprintf("Sha1 sum for %s is %s which does not match the expected %s.", filePath, sha1Actual, sha1sum))
			return err
		}
	}

	if sha256sum != "" {
		if sha256sum != sha256Actual {
			err = errors.New(fmt.Sprintf("Sha256 sum for %s is %s which does not match the expected %s.", filePath, sha256Actual, sha256sum))
			return err
		}
	}

	// write file to filesystem.
	err = ioutil.WriteFile(filePath, fileBytes, 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", filePath)
	}

	return err
}

// PutHandlerHtpasswd Handles puts with htpasswd auth
func (d *DBTRepoServer) PutHandlerHtpasswd(w http.ResponseWriter, r *auth.AuthenticatedRequest) {

	err := d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
	if err != nil {
		err = errors.Wrapf(err, "failed writing file %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// PubkeyUser A representation of a user permitted to authenticate via public key.  PubkeyUsers will have at minimum a Username, and a list of authorized public keys.
type PubkeyUser struct {
	Username      string `json:"username"`
	AuthorizedKey string `json:"publickey"`
}

// PubkeyIdpFile A representation of a public key IDP (Identity Provider) file.  Will have a list of users allowed to GET and a list of users authorized to PUT.
type PubkeyIdpFile struct {
	GetUsers []PubkeyUser `json:"getUsers"`
	PutUsers []PubkeyUser `json:"putUsers"`
}

// LoadPubkeyIdpFile Loads a public key IDP JSON file.
func LoadPubkeyIdpFile(filePath string) (pkidp PubkeyIdpFile, err error) {
	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		err = errors.Wrapf(err, "failed to read idp file %s", filePath)
		return pkidp, err
	}

	err = json.Unmarshal(fileData, &pkidp)
	if err != nil {
		err = errors.Wrapf(err, "failed to unmarshal data in %s to PubkeyIdpFile", filePath)
		return pkidp, err
	}

	return pkidp, err
}

/*
Sample Pubkey IDP File
{
	"getUsers": [
		{
			"username": "foo",
			"publickey": ""
		}
	],
	"putUsers": [
		{
			"username": "bar",
			"publickey": ""
		}
	]
}
*/

// PubkeyFromFilePut takes a subject name, and pulls the corresponding pubkey out of the identity provider file for puts
func (d *DBTRepoServer) PubkeyFromFilePut(subject string) (pubkeys string, err error) {
	idpFile, err := LoadPubkeyIdpFile(d.AuthOptsPut.IdpFile)
	if err != nil {
		err = errors.Wrapf(err, "failed loading PUT IDP file%s", d.AuthOptsPut.IdpFile)
		return pubkeys, err
	}

	for _, u := range idpFile.PutUsers {
		if u.Username == subject {
			pubkeys = u.AuthorizedKey
			log.Printf("Returning put key %q\n", pubkeys)
			return pubkeys, err
		}
	}

	err = errors.New(fmt.Sprintf("pubkey not found for %s", subject))

	return pubkeys, err
}

// PubkeyFromFileGet takes a subject name, and pulls the corresponding pubkey out of the identity provider file for puts
func (d *DBTRepoServer) PubkeyFromFileGet(subject string) (pubkeys string, err error) {
	idpFile, err := LoadPubkeyIdpFile(d.AuthOptsGet.IdpFile)
	if err != nil {
		err = errors.Wrapf(err, "failed loading GET IDP file%s", d.AuthOptsGet.IdpFile)
		return pubkeys, err
	}

	for _, u := range idpFile.GetUsers {
		if u.Username == subject {
			pubkeys = u.AuthorizedKey
			log.Printf("Returning get key %q\n", pubkeys)
			return pubkeys, err
		}
	}
	err = errors.New(fmt.Sprintf("pubkey not found for %s", subject))

	return pubkeys, err
}

// PubkeyFromFuncPut takes a subject name, and runs the configured function to return the corresponding public key
func (d *DBTRepoServer) PubkeysFromFuncPut(subject string) (pubkeys string, err error) {

	// TODO need to implement PubkeyFromFunc.

	return pubkeys, err
}

// PubkeyFromFuncGet takes a subject name, and runs the configured function to return the corresponding public key
func (d *DBTRepoServer) PubkeysFromFuncGet(subject string) (pubkeys string, err error) {

	// TODO need to implement PubkeyFromFunc.

	return pubkeys, err
}

// AuthenticatedHandlerFunc is like http.HandlerFunc, but takes AuthenticatedRequest instead of http.Request
type AuthenticatedHandlerFunc func(http.ResponseWriter, *AuthenticatedRequest)

// AuthenticatedRequest  Basically an http.Request with an added Username field.  The Username should never be empty.
type AuthenticatedRequest struct {
	http.Request
	/*
	 Authenticated user name. Current API implies that Username is
	 never empty, which means that authentication is always done
	 before calling the request handler.
	*/
	Username string
}

// CheckPubkeyAuth Function that actually checks the Token sent by the client in the headers.
func CheckPubkeyAuth(w http.ResponseWriter, r *http.Request, pubkeyRetrievalFunc func(subject string) (pubkeys string, err error)) (username string) {
	tokenString := r.Header.Get("Token")

	if tokenString == "" {
		log.Info("Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	// TODO sanity check username?

	//Parse the token, which includes setting up it's internals so it can be verified.
	subject, token, err := agentjwt.ParsePubkeySignedToken(tokenString, pubkeyRetrievalFunc)
	if err != nil {
		log.Errorf("Error parsing token: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return username
	}

	if !token.Valid {
		log.Info("Auth Failed")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	log.Infof("Subject %s successfully authenticated", subject)
	username = subject

	return username
}

// Wrap returns an http.HandlerFunc which wraps AuthenticatedHandlerFunc
func Wrap(wrapped AuthenticatedHandlerFunc, pubkeyRetrievalFunc func(subject string) (pubkeys string, err error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if username := CheckPubkeyAuth(w, r, pubkeyRetrievalFunc); username != "" {
			ar := &AuthenticatedRequest{Request: *r, Username: username}
			wrapped(w, ar)
		}
	}
}

// CheckPubkeysGetFile Checks the pubkey signature in the JWT token against a public key found in a htpasswd like file and if things check out, passes things along to the provided handler.
func (d *DBTRepoServer) CheckPubkeysGetFile(wrapped http.HandlerFunc) http.HandlerFunc {
	return Wrap(func(w http.ResponseWriter, ar *AuthenticatedRequest) {
		ar.Header.Set("X-Authenticated-Username", ar.Username)
		wrapped(w, &ar.Request)
	}, d.PubkeyFromFileGet)
}

// CheckPubkeysGetFunc Checks the pubkey signature in the JWT token against a public key produced from a function and if things check out, passes things along to the provided handler.
func (d *DBTRepoServer) CheckPubkeysGetFunc(wrapped http.HandlerFunc) http.HandlerFunc {
	return Wrap(func(w http.ResponseWriter, ar *AuthenticatedRequest) {
		ar.Header.Set("X-Authenticated-Username", ar.Username)
		wrapped(w, &ar.Request)
	}, d.PubkeysFromFuncGet)
}

// PutHandlerPubKeyFile
func (d *DBTRepoServer) PutHandlerPubkeyFile(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Token")

	fmt.Printf("Put Token from server: %q\n", tokenString)

	if tokenString == "" {
		log.Info("Put Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// TODO sanity check username?

	// Parse the token, which includes setting up it's internals so it can be verified.
	subject, token, err := agentjwt.ParsePubkeySignedToken(tokenString, d.PubkeyFromFilePut)
	if err != nil {
		log.Errorf("Error: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !token.Valid {
		log.Info("Auth Failed")
		w.WriteHeader(http.StatusUnauthorized)
	}

	log.Infof("Subject %s successfully authenticated", subject)

	err = d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
	if err != nil {
		err = errors.Wrapf(err, "failed writing file %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// PutHandlerPubkeyFunc
func (d *DBTRepoServer) PutHandlerPubkeyFunc(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Token")

	if tokenString == "" {
		log.Info("Put Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// TODO sanity check username?

	//Parse the token, which includes setting up it's internals so it can be verified.
	subject, token, err := agentjwt.ParsePubkeySignedToken(tokenString, d.PubkeysFromFuncPut)
	if err != nil {
		log.Errorf("Error: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !token.Valid {
		log.Info("Auth Failed")
		w.WriteHeader(http.StatusUnauthorized)
	}

	log.Infof("Subject %s successfully authenticated", subject)

	err = d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
	if err != nil {
		err = errors.Wrapf(err, "failed writing file %s", r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
		log.Error(err)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

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
