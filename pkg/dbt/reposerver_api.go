package dbt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"time"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// ToolInfo represents basic information about a tool.
type ToolInfo struct {
	Name string `json:"name"`
}

// VersionInfo represents version information including modification time.
type VersionInfo struct {
	Version    string    `json:"version"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// APIToolsHandler returns a JSON list of available tools.
func (d *DBTRepoServer) APIToolsHandler(w http.ResponseWriter, r *http.Request) {
	toolsDir := fmt.Sprintf("%s/dbt-tools", d.ServerRoot)

	entries, readErr := os.ReadDir(toolsDir)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, writeErr := w.Write([]byte("[]"))
			if writeErr != nil {
				log.Errorf("failed to write response: %v", writeErr)
			}
			return
		}
		log.Errorf("failed to read tools directory %s: %v", toolsDir, readErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	tools := make([]ToolInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			tools = append(tools, ToolInfo{Name: entry.Name()})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encodeErr := json.NewEncoder(w).Encode(tools)
	if encodeErr != nil {
		log.Errorf("failed to encode tools response: %v", encodeErr)
	}
}

// APIToolVersionsHandler returns a JSON list of versions for a tool with modification times.
func (d *DBTRepoServer) APIToolVersionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	toolName := vars["name"]

	if toolName == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	versionsDir := fmt.Sprintf("%s/dbt-tools/%s", d.ServerRoot, toolName)

	entries, readErr := os.ReadDir(versionsDir)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Errorf("failed to read versions directory %s: %v", versionsDir, readErr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	semverMatch := regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	versions := make([]VersionInfo, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if !semverMatch.MatchString(entry.Name()) {
			continue
		}

		info, infoErr := entry.Info()
		if infoErr != nil {
			log.Errorf("failed to stat version directory %s/%s: %v", versionsDir, entry.Name(), infoErr)
			continue
		}

		versions = append(versions, VersionInfo{
			Version:    entry.Name(),
			ModifiedAt: info.ModTime(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encodeErr := json.NewEncoder(w).Encode(versions)
	if encodeErr != nil {
		log.Errorf("failed to encode versions response: %v", encodeErr)
	}
}

// setupAPIRoutes configures the API routes under /-/api/.
// These routes use GET auth configuration since they are read-only.
// Supports comma-separated auth types for multi-auth (e.g. "static-token,oidc").
func (d *DBTRepoServer) setupAPIRoutes(r *mux.Router, oidcValidator *OIDCValidator) (err error) {
	apiRouter := r.PathPrefix("/-/api").Subrouter()

	if d.AuthTypeGet == "" || !d.AuthGets {
		apiRouter.HandleFunc("/tools", d.APIToolsHandler).Methods("GET")
		apiRouter.HandleFunc("/tools/{name}/versions", d.APIToolVersionsHandler).Methods("GET")
		return err
	}

	authTypes := parseAuthTypes(d.AuthTypeGet)
	if len(authTypes) > 1 {
		check, buildErr := d.buildMultiAuthCheck(authTypes, d.AuthOptsGet, oidcValidator, false)
		if buildErr != nil {
			err = buildErr
			return err
		}

		apiRouter.Handle("/tools", d.CheckMultiAuthGet(d.APIToolsHandler, check)).Methods("GET")
		apiRouter.Handle("/tools/{name}/versions", d.CheckMultiAuthGet(d.APIToolVersionsHandler, check)).Methods("GET")

		return err
	}

	err = d.registerAuthenticatedAPIRoutes(apiRouter, oidcValidator)

	return err
}

// registerAuthenticatedAPIRoutes registers API routes with authentication.
func (d *DBTRepoServer) registerAuthenticatedAPIRoutes(apiRouter *mux.Router, oidcValidator *OIDCValidator) (err error) {
	switch d.AuthTypeGet {
	case AUTH_BASIC_HTPASSWD:
		htpasswd := auth.HtpasswdFileProvider(d.AuthOptsGet.IdpFile)
		authenticator := auth.NewBasicAuthenticator("DBT Server", htpasswd)
		apiRouter.HandleFunc("/tools", authenticator.Wrap(d.wrapHTPasswdAPI(d.APIToolsHandler))).Methods("GET")
		apiRouter.HandleFunc("/tools/{name}/versions", authenticator.Wrap(d.wrapHTPasswdAPI(d.APIToolVersionsHandler))).Methods("GET")
	case AUTH_SSH_AGENT_FILE:
		apiRouter.Handle("/tools", d.CheckPubkeysGetFile(d.APIToolsHandler)).Methods("GET")
		apiRouter.Handle("/tools/{name}/versions", d.CheckPubkeysGetFile(d.APIToolVersionsHandler)).Methods("GET")
	case AUTH_SSH_AGENT_FUNC:
		apiRouter.Handle("/tools", d.CheckPubkeysGetFunc(d.APIToolsHandler)).Methods("GET")
		apiRouter.Handle("/tools/{name}/versions", d.CheckPubkeysGetFunc(d.APIToolVersionsHandler)).Methods("GET")
	case AUTH_OIDC:
		apiRouter.Handle("/tools", d.CheckOIDCGet(d.APIToolsHandler, oidcValidator)).Methods("GET")
		apiRouter.Handle("/tools/{name}/versions", d.CheckOIDCGet(d.APIToolVersionsHandler, oidcValidator)).Methods("GET")
	case AUTH_STATIC_TOKEN:
		apiRouter.Handle("/tools", d.CheckStaticTokenGet(d.APIToolsHandler)).Methods("GET")
		apiRouter.Handle("/tools/{name}/versions", d.CheckStaticTokenGet(d.APIToolVersionsHandler)).Methods("GET")
	default:
		err = errors.New(fmt.Sprintf("unsupported auth method for API: %s", d.AuthTypeGet))
		return err
	}

	return err
}

// wrapHTPasswdAPI wraps a standard http.HandlerFunc into an htpasswd-compatible handler.
func (d *DBTRepoServer) wrapHTPasswdAPI(handler http.HandlerFunc) (wrapped func(http.ResponseWriter, *auth.AuthenticatedRequest)) {
	wrapped = func(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
		handler(w, &r.Request)
	}
	return wrapped
}
