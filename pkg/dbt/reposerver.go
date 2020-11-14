package dbt

import (
	"encoding/json"
	"fmt"
	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/nikogura/gomason/pkg/gomason"
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
	Address    string   `json:"address"`
	Port       int      `json:"port"`
	ServerRoot string   `json:"serverRoot"`
	AuthType   string   `json:"authType"`
	AuthGets   bool     `json:"authGets"`
	AuthOpts   AuthOpts `json:"authOpts"`
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
	if d.AuthType != "" {
		switch d.AuthType {
		case AUTH_BASIC_HTPASSWD:
			htpasswd := auth.HtpasswdFileProvider(d.AuthOpts.IdpFile)
			authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
			r.PathPrefix("/").HandlerFunc(authenticator.Wrap(d.PutHandlerHtpasswd)).Methods("PUT")
		//case AUTH_SSH_AGENT_FILE:
		//	r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFile).Methods("PUT")
		//case AUTH_SSH_AGENT_FUNC:
		//	r.PathPrefix("/").HandlerFunc(d.PutHandlerPubkeyFunc).Methods("PUT")
		//case AUTH_BASIC_LDAP:
		//	err = errors.New("basic auth via ldap not yet supported")
		//	return err
		//case AUTH_SSH_AGENT_LDAP:
		//	err = errors.New("ssh-agent auth via ldap not yet supported")
		//	return err
		default:
			err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthType))
			return err
		}
	}

	// handle the downloads and indices
	if d.AuthType != "" && d.AuthGets {
		switch d.AuthType {
		case AUTH_BASIC_HTPASSWD:
			htpasswd := auth.HtpasswdFileProvider(d.AuthOpts.IdpFile)
			authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
			r.PathPrefix("/").Handler(auth.JustCheck(authenticator, http.FileServer(http.Dir(d.ServerRoot)).ServeHTTP)).Methods("GET", "HEAD")
		//case AUTH_SSH_AGENT_FILE:
		//	err = errors.New("pubkey auth via file not yet supported")
		//	return err
		//
		//case AUTH_SSH_AGENT_FUNC:
		//	err = errors.New("pubkey auth via file not yet supported")
		//	return err
		//
		//case AUTH_BASIC_LDAP:
		//	err = errors.New("basic auth via ldap not yet supported")
		//	return err
		//
		//case AUTH_SSH_AGENT_LDAP:
		//	err = errors.New("ssh-agent auth via ldap not yet supported")
		//	return err
		//
		default:
			err = errors.New(fmt.Sprintf("unsupported auth method: %s", d.AuthType))
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
//	log.Infof("Subject %s successfully authenticated", subject)
//
//	err = d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
//	if err != nil {
//		err = errors.Wrapf(err, "failed writing file %s", r.URL.Path)
//		w.WriteHeader(http.StatusInternalServerError)
//		log.Error(err)
//		return
//	}
//
//	w.WriteHeader(http.StatusCreated)
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
//	log.Infof("Subject %s successfully authenticated", subject)
//
//	err = d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
//	if err != nil {
//		err = errors.Wrapf(err, "failed writing file %s", r.URL.Path)
//		w.WriteHeader(http.StatusInternalServerError)
//		log.Error(err)
//		return
//	}
//
//	w.WriteHeader(http.StatusCreated)
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
