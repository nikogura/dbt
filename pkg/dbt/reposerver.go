package dbt

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/nikogura/gomason/pkg/gomason"
	"github.com/nikogura/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

//nolint:revive,staticcheck // AUTH_BASIC_HTPASSWD is a public API constant
const AUTH_BASIC_HTPASSWD = "basic-htpasswd"

//nolint:revive,staticcheck // AUTH_SSH_AGENT_FILE is a public API constant
const AUTH_SSH_AGENT_FILE = "ssh-agent-file"

//nolint:revive,staticcheck // AUTH_SSH_AGENT_FUNC is a public API constant
const AUTH_SSH_AGENT_FUNC = "ssh-agent-func"

//nolint:revive,staticcheck // AUTH_BASIC_LDAP is a public API constant
const AUTH_BASIC_LDAP = "basic-ldap"

//nolint:revive,staticcheck // AUTH_SSH_AGENT_LDAP is a public API constant
const AUTH_SSH_AGENT_LDAP = "ssh-agent-ldap"

//nolint:gochecknoinits // logrus configuration needs to happen at package init
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

// AuthOpts is a struct for holding Auth options.
type AuthOpts struct {
	IdpFile string        `json:"idpFile"`
	IdpFunc string        `json:"idpFunc,omitempty"`
	OIDC    *OIDCAuthOpts `json:"oidc,omitempty"`
}

// NewRepoServer creates a new DBTRepoServer object from the config file provided.
func NewRepoServer(configFilePath string) (server *DBTRepoServer, err error) {
	c, readErr := os.ReadFile(configFilePath)
	if readErr != nil {
		err = errors.Wrapf(readErr, "failed to read config file %q", configFilePath)
		return server, err
	}

	server = &DBTRepoServer{}

	err = json.Unmarshal(c, server)
	if err != nil {
		err = errors.Wrapf(err, "failed to unmarshal json in %q", configFilePath)
	}

	return server, err
}

// initOIDCValidators creates OIDC validators for PUT and GET if configured.
func (d *DBTRepoServer) initOIDCValidators() (putValidator *OIDCValidator, getValidator *OIDCValidator, err error) {
	if d.AuthTypePut == AUTH_OIDC {
		if d.AuthOptsPut.OIDC == nil {
			err = errors.New("OIDC auth type requires oidc configuration in authOptsPut")
			return putValidator, getValidator, err
		}
		putValidator, err = NewOIDCValidator(context.Background(), d.AuthOptsPut.OIDC)
		if err != nil {
			err = errors.Wrap(err, "failed to create OIDC validator for PUT")
			return putValidator, getValidator, err
		}
	}

	if d.AuthTypeGet == AUTH_OIDC && d.AuthGets {
		if d.AuthOptsGet.OIDC == nil {
			err = errors.New("OIDC auth type requires oidc configuration in authOptsGet")
			return putValidator, getValidator, err
		}
		getValidator, err = NewOIDCValidator(context.Background(), d.AuthOptsGet.OIDC)
		if err != nil {
			err = errors.Wrap(err, "failed to create OIDC validator for GET")
			return putValidator, getValidator, err
		}
	}

	return putValidator, getValidator, err
}

// setupPutRoutes configures PUT routes based on auth type.
func (d *DBTRepoServer) setupPutRoutes(r *mux.Router, oidcValidator *OIDCValidator) (err error) {
	if d.AuthTypePut == "" {
		return err
	}

	switch d.AuthTypePut {
	case AUTH_BASIC_HTPASSWD:
		htpasswd := auth.HtpasswdFileProvider(d.AuthOptsPut.IdpFile)
		authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
		r.PathPrefix("/").HandlerFunc(authenticator.Wrap(d.PutHandlerHtpasswd)).Methods("PUT")
	case AUTH_SSH_AGENT_FILE:
		r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFile).Methods("PUT")
	case AUTH_SSH_AGENT_FUNC:
		r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFunc).Methods("PUT")
	case AUTH_OIDC:
		r.PathPrefix("/").HandlerFunc(d.PutHandlerOIDC(oidcValidator)).Methods("PUT")
	default:
		err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthTypePut))
		return err
	}

	return err
}

// setupGetRoutes configures GET routes based on auth type.
func (d *DBTRepoServer) setupGetRoutes(r *mux.Router, oidcValidator *OIDCValidator) (err error) {
	fileServer := http.FileServer(http.Dir(d.ServerRoot))

	if d.AuthTypeGet == "" || !d.AuthGets {
		r.PathPrefix("/").Handler(fileServer).Methods("GET", "HEAD")
		return err
	}

	switch d.AuthTypeGet {
	case AUTH_BASIC_HTPASSWD:
		htpasswd := auth.HtpasswdFileProvider(d.AuthOptsGet.IdpFile)
		authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
		r.PathPrefix("/").Handler(auth.JustCheck(authenticator, fileServer.ServeHTTP)).Methods("GET", "HEAD")
	case AUTH_SSH_AGENT_FILE:
		r.PathPrefix("/").Handler(d.CheckPubkeysGetFile(fileServer.ServeHTTP)).Methods("GET", "HEAD")
	case AUTH_SSH_AGENT_FUNC:
		r.PathPrefix("/").Handler(d.CheckPubkeysGetFunc(fileServer.ServeHTTP)).Methods("GET", "HEAD")
	case AUTH_OIDC:
		r.PathPrefix("/").Handler(d.CheckOIDCGet(fileServer.ServeHTTP, oidcValidator)).Methods("GET", "HEAD")
	default:
		err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthTypeGet))
		return err
	}

	return err
}

// HealthHandler handles health check requests without authentication.
func (d *DBTRepoServer) HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, writeErr := w.Write([]byte("ok"))
	if writeErr != nil {
		log.Errorf("failed to write health check response: %v", writeErr)
	}
}

// RunRepoServer Run runs the test repository server.
func (d *DBTRepoServer) RunRepoServer() (err error) {
	log.Printf("Running dbt artifact server on %s port %d.  Serving tree at: %s", d.Address, d.Port, d.ServerRoot)

	fullAddress := fmt.Sprintf("%s:%s", d.Address, strconv.Itoa(d.Port))
	r := mux.NewRouter()

	// Health check endpoint - no auth required
	r.HandleFunc("/healthz", d.HealthHandler).Methods("GET", "HEAD")

	// Initialize OIDC validators if needed
	oidcValidatorPut, oidcValidatorGet, initErr := d.initOIDCValidators()
	if initErr != nil {
		err = initErr
		return err
	}

	// Setup PUT routes
	err = d.setupPutRoutes(r, oidcValidatorPut)
	if err != nil {
		return err
	}

	// Setup GET routes
	err = d.setupGetRoutes(r, oidcValidatorGet)
	if err != nil {
		return err
	}

	// Run the server
	err = http.ListenAndServe(fullAddress, r)
	return err
}

func (d *DBTRepoServer) HandlePut(path string, body io.ReadCloser, md5sum string, sha1sum string, sha256sum string) (err error) {
	filePath := fmt.Sprintf("%s/%s", d.ServerRoot, path)
	fileDir := filepath.Dir(filePath)

	// create subdirs if they don't exist
	_, statErr := os.Stat(fileDir)
	if os.IsNotExist(statErr) {
		mkdirErr := os.MkdirAll(fileDir, 0755)
		if mkdirErr != nil {
			err = errors.Wrapf(mkdirErr, "failed to create server path %s", fileDir)
			return err
		}
	}

	fileBytes, readErr := io.ReadAll(body)
	if readErr != nil {
		err = errors.Wrapf(readErr, "failed to read request body")
		return err
	}

	// Checksum bytes
	md5Actual, sha1Actual, sha256Actual, checksumErr := gomason.AllChecksumsForBytes(fileBytes)
	if checksumErr != nil {
		err = errors.Wrapf(checksumErr, "failed to derive checksums for file %s", filePath)
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
	err = os.WriteFile(filePath, fileBytes, 0644)
	if err != nil {
		err = errors.Wrapf(err, "failed to write %s", filePath)
	}

	return err
}

// PutHandlerHtpasswd handles puts with htpasswd auth.
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
	fileData, readErr := os.ReadFile(filePath)
	if readErr != nil {
		err = errors.Wrapf(readErr, "failed to read idp file %s", filePath)
		return pkidp, err
	}

	err = json.Unmarshal(fileData, &pkidp)
	if err != nil {
		err = errors.Wrapf(err, "failed to unmarshal data in %s to PubkeyIdpFile", filePath)
		return pkidp, err
	}

	return pkidp, err
}

// PubkeyFromFilePut takes a subject name and pulls the corresponding pubkey out of the identity provider file for puts.
func (d *DBTRepoServer) PubkeyFromFilePut(subject string) (pubkeys []string, err error) {
	idpFile, loadErr := LoadPubkeyIdpFile(d.AuthOptsPut.IdpFile)
	if loadErr != nil {
		err = errors.Wrapf(loadErr, "failed loading PUT IDP file%s", d.AuthOptsPut.IdpFile)
		return pubkeys, err
	}

	pubkeys = make([]string, 0)

	for _, u := range idpFile.PutUsers {
		if u.Username == subject {
			pubkeys = append(pubkeys, u.AuthorizedKey)
			log.Printf("Returning put key %q\n", pubkeys)
			return pubkeys, err
		}
	}

	err = errors.New(fmt.Sprintf("pubkey not found for %s", subject))

	return pubkeys, err
}

// PubkeyFromFileGet takes a subject name and pulls the corresponding pubkey out of the identity provider file for gets.
func (d *DBTRepoServer) PubkeyFromFileGet(subject string) (pubkeys []string, err error) {
	idpFile, loadErr := LoadPubkeyIdpFile(d.AuthOptsGet.IdpFile)
	if loadErr != nil {
		err = errors.Wrapf(loadErr, "failed loading GET IDP file%s", d.AuthOptsGet.IdpFile)
		return pubkeys, err
	}

	pubkeys = make([]string, 0)

	for _, u := range idpFile.GetUsers {
		if u.Username == subject {
			pubkeys = append(pubkeys, u.AuthorizedKey)
			log.Printf("Returning get key %q\n", pubkeys)
			return pubkeys, err
		}
	}
	err = errors.New(fmt.Sprintf("pubkey not found for %s", subject))

	return pubkeys, err
}

// PubkeysFromFuncPut takes a subject name and runs the configured function to return the corresponding public key.
func (d *DBTRepoServer) PubkeysFromFuncPut(subject string) (pubkeys []string, err error) {
	var funcErr error
	pubkeys, funcErr = GetFuncUsername(d.AuthOptsPut.IdpFunc, subject)
	if funcErr != nil {
		err = errors.Wrapf(funcErr, "failed to get password from shell function %q", d.AuthOptsPut.IdpFunc)
		return pubkeys, err
	}

	return pubkeys, err
}

// PubkeysFromFuncGet takes a subject name and runs the configured function to return the corresponding public key.
func (d *DBTRepoServer) PubkeysFromFuncGet(subject string) (pubkey []string, err error) {
	var funcErr error
	pubkey, funcErr = GetFuncUsername(d.AuthOptsGet.IdpFunc, subject)
	if funcErr != nil {
		err = errors.Wrapf(funcErr, "failed to get password from shell function %q", d.AuthOptsGet.IdpFunc)
		return pubkey, err
	}

	return pubkey, err
}

// AuthenticatedHandlerFunc is like http.HandlerFunc, but takes AuthenticatedRequest instead of http.Request.
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
func CheckPubkeyAuth(w http.ResponseWriter, r *http.Request, audience string, pubkeyRetrievalFunc func(subject string) (pubkeys []string, err error)) (username string) {
	tokenString := r.Header.Get("Token")

	if tokenString == "" {
		log.Info("Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	// Wrap the logrus logger so we can use it in our retrieval func
	logAdapter := &LogrusAdapter{
		logger: log.New(),
	}

	//Parse the token, which includes setting up it's internals so it can be verified.
	subject, token, err := agentjwt.VerifyToken(tokenString, []string{audience}, pubkeyRetrievalFunc, logAdapter)
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

// Wrap returns an http.HandlerFunc which wraps AuthenticatedHandlerFunc.
func Wrap(wrapped AuthenticatedHandlerFunc, audience string, pubkeyRetrievalFunc func(subject string) (pubkeys []string, err error)) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		if username := CheckPubkeyAuth(w, r, audience, pubkeyRetrievalFunc); username != "" {
			ar := &AuthenticatedRequest{Request: *r, Username: username}
			wrapped(w, ar)
		}
	}
	return handler
}

// CheckPubkeysGetFile Checks the pubkey signature in the JWT token against a public key found in a htpasswd like file and if things check out, passes things along to the provided handler.
func (d *DBTRepoServer) CheckPubkeysGetFile(wrapped http.HandlerFunc) (handler http.HandlerFunc) {
	// Extract the domain from the repo server
	domain, _ := ExtractDomain(d.Address)

	handler = Wrap(func(w http.ResponseWriter, ar *AuthenticatedRequest) {
		ar.Header.Set("X-Authenticated-Username", ar.Username)
		wrapped(w, &ar.Request)
	}, domain, d.PubkeyFromFileGet)
	return handler
}

// CheckPubkeysGetFunc Checks the pubkey signature in the JWT token against a public key produced from a function and if things check out, passes things along to the provided handler.
func (d *DBTRepoServer) CheckPubkeysGetFunc(wrapped http.HandlerFunc) (handler http.HandlerFunc) {
	// Extract the domain from the repo server
	domain, _ := ExtractDomain(d.Address)

	handler = Wrap(func(w http.ResponseWriter, ar *AuthenticatedRequest) {
		ar.Header.Set("X-Authenticated-Username", ar.Username)
		wrapped(w, &ar.Request)
	}, domain, d.PubkeysFromFuncGet)
	return handler
}

// PutHandlerPubkeyFile handles PUT requests with public key file authentication.
func (d *DBTRepoServer) PutHandlerPubkeyFile(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Token")

	fmt.Printf("Put Token from server: %q\n", tokenString)

	if tokenString == "" {
		log.Info("Put Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// TODO sanity check username?

	// Extract the domain from the repo server
	domain, domainErr := ExtractDomain(d.Address)
	if domainErr != nil {
		log.Errorf("failed extracting domain from configured dbt repo url %s: %v", d.Address, domainErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Wrap the logrus logger so we can use it in our retrieval func
	logAdapter := &LogrusAdapter{
		logger: log.New(),
	}

	// Parse the token, which includes setting up it's internals so it can be verified.
	subject, token, verifyErr := agentjwt.VerifyToken(tokenString, []string{domain}, d.PubkeyFromFilePut, logAdapter)
	if verifyErr != nil {
		log.Errorf("Error: %s", verifyErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !token.Valid {
		log.Info("Auth Failed")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Infof("Subject %s successfully authenticated", subject)

	putErr := d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
	if putErr != nil {
		log.Errorf("failed writing file %s: %v", r.URL.Path, putErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// PutHandlerPubkeyFunc handles PUT requests with public key function authentication.
func (d *DBTRepoServer) PutHandlerPubkeyFunc(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Token")

	if tokenString == "" {
		log.Info("Put Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// TODO sanity check username?

	// Extract the domain from the repo server.
	domain, domainErr := ExtractDomain(d.Address)
	if domainErr != nil {
		log.Errorf("failed extracting domain from configured dbt repo url %s: %v", d.Address, domainErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Wrap the logrus logger so we can use it in our retrieval func.
	logAdapter := &LogrusAdapter{
		logger: log.New(),
	}

	// Parse the token, which includes setting up it's internals so it can be verified.
	subject, token, verifyErr := agentjwt.VerifyToken(tokenString, []string{domain}, d.PubkeysFromFuncPut, logAdapter)
	if verifyErr != nil {
		log.Errorf("Error: %s", verifyErr)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !token.Valid {
		log.Info("Auth Failed")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	log.Infof("Subject %s successfully authenticated", subject)

	putErr := d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
	if putErr != nil {
		log.Errorf("failed writing file %s: %v", r.URL.Path, putErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

type Logger interface {
	Debug(msg string, args ...any)
}

type LogrusAdapter struct {
	logger *log.Logger
}

func (l *LogrusAdapter) Debug(msg string, args ...any) {
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}

	l.logger.Debug(msg)
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
