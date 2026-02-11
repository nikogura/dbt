package dbt

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/nikogura/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// ValidatePathWithinRoot validates that the resolved path is within the server root.
// Returns an error if the path attempts directory traversal outside the root.
func (d *DBTRepoServer) ValidatePathWithinRoot(requestPath string) (resolvedPath string, err error) {
	fullPath := filepath.Join(d.ServerRoot, requestPath)

	var absErr error
	resolvedPath, absErr = filepath.Abs(filepath.Clean(fullPath))
	if absErr != nil {
		err = errors.Wrapf(absErr, "failed to resolve path %s", requestPath)
		return resolvedPath, err
	}

	absRoot, rootErr := filepath.Abs(d.ServerRoot)
	if rootErr != nil {
		err = errors.Wrapf(rootErr, "failed to resolve server root %s", d.ServerRoot)
		return resolvedPath, err
	}

	if !strings.HasPrefix(resolvedPath, absRoot) {
		err = fmt.Errorf("path %s is outside server root", requestPath)
		return resolvedPath, err
	}

	return resolvedPath, err
}

// HandleDelete handles deletion of a file or directory at the given path.
func (d *DBTRepoServer) HandleDelete(requestPath string) (statusCode int, err error) {
	resolvedPath, validateErr := d.ValidatePathWithinRoot(requestPath)
	if validateErr != nil {
		statusCode = http.StatusForbidden
		err = errors.Wrapf(validateErr, "path validation failed for %s", requestPath)
		return statusCode, err
	}

	info, statErr := os.Stat(resolvedPath)
	if os.IsNotExist(statErr) {
		statusCode = http.StatusNotFound
		err = fmt.Errorf("path %s not found", requestPath)
		return statusCode, err
	}

	if statErr != nil {
		statusCode = http.StatusInternalServerError
		err = errors.Wrapf(statErr, "failed to stat %s", resolvedPath)
		return statusCode, err
	}

	if info.IsDir() {
		removeErr := os.RemoveAll(resolvedPath)
		if removeErr != nil {
			statusCode = http.StatusInternalServerError
			err = errors.Wrapf(removeErr, "failed to remove directory %s", resolvedPath)
			return statusCode, err
		}
	} else {
		removeErr := os.Remove(resolvedPath)
		if removeErr != nil {
			statusCode = http.StatusInternalServerError
			err = errors.Wrapf(removeErr, "failed to remove file %s", resolvedPath)
			return statusCode, err
		}
	}

	statusCode = http.StatusNoContent

	return statusCode, err
}

// setupDeleteRoutes configures DELETE routes based on auth type.
// DELETE reuses PUT auth configuration since it is a write operation.
//
//nolint:dupl // mirrors setupPutRoutes intentionally; they must evolve in parallel
func (d *DBTRepoServer) setupDeleteRoutes(r *mux.Router, oidcValidator *OIDCValidator) (err error) {
	if d.AuthTypePut == "" {
		return err
	}

	switch d.AuthTypePut {
	case AUTH_BASIC_HTPASSWD:
		htpasswd := auth.HtpasswdFileProvider(d.AuthOptsPut.IdpFile)
		authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
		r.PathPrefix("/").HandlerFunc(authenticator.Wrap(d.DeleteHandlerHtpasswd)).Methods("DELETE")
	case AUTH_SSH_AGENT_FILE:
		r.PathPrefix("/").HandlerFunc(d.DeleteHandlerPubkeyFile).Methods("DELETE")
	case AUTH_SSH_AGENT_FUNC:
		r.PathPrefix("/").HandlerFunc(d.DeleteHandlerPubkeyFunc).Methods("DELETE")
	case AUTH_OIDC:
		r.PathPrefix("/").HandlerFunc(d.DeleteHandlerOIDC(oidcValidator)).Methods("DELETE")
	case AUTH_STATIC_TOKEN:
		r.PathPrefix("/").HandlerFunc(d.DeleteHandlerStaticToken()).Methods("DELETE")
	default:
		err = errors.New(fmt.Sprintf("unsupported auth method for DELETE: %s", d.AuthTypePut))
		return err
	}

	return err
}

// deleteAndRespond performs the delete and writes the HTTP response.
func (d *DBTRepoServer) deleteAndRespond(w http.ResponseWriter, requestPath string) {
	statusCode, deleteErr := d.HandleDelete(requestPath)
	if deleteErr != nil {
		log.Errorf("DELETE %s: %v", requestPath, deleteErr)
		w.WriteHeader(statusCode)
		return
	}

	w.WriteHeader(statusCode)
}

// DeleteHandlerHtpasswd handles DELETE requests with htpasswd auth.
func (d *DBTRepoServer) DeleteHandlerHtpasswd(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	d.deleteAndRespond(w, r.URL.Path)
}

// verifyPubkeyAndDelete authenticates a DELETE request using public key verification
// and performs the deletion if authentication succeeds.
func (d *DBTRepoServer) verifyPubkeyAndDelete(w http.ResponseWriter, r *http.Request, pubkeyFunc func(string) ([]string, error)) {
	tokenString := r.Header.Get("Token")

	if tokenString == "" {
		log.Info("Delete Auth Failed: no token provided.")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	domain, domainErr := ExtractDomain(d.Address)
	if domainErr != nil {
		log.Errorf("failed extracting domain from configured dbt repo url %s: %v", d.Address, domainErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	logAdapter := &LogrusAdapter{
		logger: log.New(),
	}

	subject, token, verifyErr := agentjwt.VerifyToken(tokenString, []string{domain}, pubkeyFunc, logAdapter)
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

	log.Infof("Subject %s successfully authenticated for DELETE", subject)

	d.deleteAndRespond(w, r.URL.Path)
}

// DeleteHandlerPubkeyFile handles DELETE requests with public key file authentication.
func (d *DBTRepoServer) DeleteHandlerPubkeyFile(w http.ResponseWriter, r *http.Request) {
	d.verifyPubkeyAndDelete(w, r, d.PubkeyFromFilePut)
}

// DeleteHandlerPubkeyFunc handles DELETE requests with public key function authentication.
func (d *DBTRepoServer) DeleteHandlerPubkeyFunc(w http.ResponseWriter, r *http.Request) {
	d.verifyPubkeyAndDelete(w, r, d.PubkeysFromFuncPut)
}

// DeleteHandlerOIDC handles DELETE requests with OIDC authentication.
func (d *DBTRepoServer) DeleteHandlerOIDC(validator *OIDCValidator) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		username := CheckOIDCAuth(w, r, validator)
		if username == "" {
			return
		}

		log.Infof("OIDC Subject %s successfully authenticated for DELETE", username)

		d.deleteAndRespond(w, r.URL.Path)
	}
	return handler
}

// DeleteHandlerStaticToken handles DELETE requests with static token authentication.
func (d *DBTRepoServer) DeleteHandlerStaticToken() (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		expectedToken := d.getStaticTokenPut()
		username := CheckStaticTokenAuth(w, r, expectedToken)
		if username == "" {
			return
		}

		log.Infof("Static Token user authenticated for DELETE")

		d.deleteAndRespond(w, r.URL.Path)
	}
	return handler
}
