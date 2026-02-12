package dbt

import (
	"net/http"
	"os"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/nikogura/jwt-ssh-agent-go/pkg/agentjwt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// multiAuthChecker tries authentication and returns the username if successful.
type multiAuthChecker func(r *http.Request) (username string)

// parseAuthTypes splits a comma-separated auth type string into individual types.
func parseAuthTypes(authType string) (types []string) {
	raw := strings.Split(authType, ",")
	types = make([]string, 0, len(raw))

	for _, t := range raw {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" {
			types = append(types, trimmed)
		}
	}

	return types
}

// containsAuthType checks if a target auth type appears in a comma-separated auth type string.
func containsAuthType(authTypeStr string, target string) (found bool) {
	for _, t := range parseAuthTypes(authTypeStr) {
		if t == target {
			found = true
			return found
		}
	}

	return found
}

// resolveStaticToken retrieves the static token from AuthOpts, preferring env var.
func resolveStaticToken(opts AuthOpts) (token string) {
	if opts.StaticTokenEnv != "" {
		token = os.Getenv(opts.StaticTokenEnv)
		if token != "" {
			return token
		}
	}

	token = opts.StaticToken

	return token
}

// TryStaticTokenAuth attempts static token authentication without writing to the response.
// Returns the username if authenticated, empty string otherwise.
func TryStaticTokenAuth(r *http.Request, expectedToken string) (username string) {
	if expectedToken == "" {
		return username
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return username
	}

	providedToken := strings.TrimPrefix(authHeader, "Bearer ")
	if providedToken == "" {
		return username
	}

	if !constantTimeCompare(providedToken, expectedToken) {
		return username
	}

	username = AUTH_STATIC_TOKEN
	log.Info("Static Token: successfully authenticated (multi-auth)")

	return username
}

// TryPubkeyAuth attempts SSH pubkey JWT authentication without writing to the response.
func TryPubkeyAuth(r *http.Request, domain string, pubkeyFunc func(string) ([]string, error)) (username string) {
	tokenString := r.Header.Get("Token")
	if tokenString == "" {
		return username
	}

	logAdapter := &LogrusAdapter{logger: log.New()}

	subject, token, verifyErr := agentjwt.VerifyToken(tokenString, []string{domain}, pubkeyFunc, logAdapter)
	if verifyErr != nil {
		return username
	}

	if !token.Valid {
		return username
	}

	username = subject
	log.Infof("Subject %s successfully authenticated via pubkey (multi-auth)", subject)

	return username
}

// pubkeyLookupFromFile creates a pubkey retrieval function from an IDP file.
func pubkeyLookupFromFile(idpFile string, isPut bool) (lookupFunc func(string) ([]string, error)) {
	lookupFunc = func(subject string) (pubkeys []string, err error) {
		pkidp, loadErr := LoadPubkeyIdpFile(idpFile)
		if loadErr != nil {
			err = errors.Wrapf(loadErr, "failed loading IDP file %s", idpFile)
			return pubkeys, err
		}

		users := pkidp.GetUsers
		if isPut {
			users = pkidp.PutUsers
		}

		pubkeys = make([]string, 0)

		for _, u := range users {
			if u.Username == subject {
				pubkeys = append(pubkeys, u.AuthorizedKey)
				return pubkeys, err
			}
		}

		err = errors.Errorf("pubkey not found for %s", subject)

		return pubkeys, err
	}

	return lookupFunc
}

// buildMultiAuthCheck creates a combined auth checker from multiple auth types.
func (d *DBTRepoServer) buildMultiAuthCheck(authTypes []string, opts AuthOpts, oidcValidator *OIDCValidator, isPut bool) (check multiAuthChecker, err error) {
	checkers := make([]multiAuthChecker, 0, len(authTypes))

	for _, authType := range authTypes {
		checker, buildErr := d.buildSingleChecker(authType, opts, oidcValidator, isPut)
		if buildErr != nil {
			err = buildErr
			return check, err
		}

		checkers = append(checkers, checker)
	}

	check = func(r *http.Request) (username string) {
		for _, c := range checkers {
			username = c(r)
			if username != "" {
				return username
			}
		}

		log.Info("Multi-auth: all authentication methods failed")

		return username
	}

	return check, err
}

// buildSingleChecker creates a single auth checker closure for the given auth type.
func (d *DBTRepoServer) buildSingleChecker(authType string, opts AuthOpts, oidcValidator *OIDCValidator, isPut bool) (checker multiAuthChecker, err error) {
	switch authType {
	case AUTH_STATIC_TOKEN:
		checker = buildStaticTokenChecker(opts)
	case AUTH_OIDC:
		checker = buildOIDCChecker(oidcValidator)
	case AUTH_BASIC_HTPASSWD:
		checker = buildBasicHtpasswdChecker(opts)
	case AUTH_SSH_AGENT_FILE:
		checker, err = d.buildPubkeyFileChecker(opts, isPut)
	case AUTH_SSH_AGENT_FUNC:
		checker, err = d.buildPubkeyFuncChecker(opts)
	default:
		err = errors.Errorf("unsupported auth type in multi-auth: %s", authType)
	}

	return checker, err
}

// buildStaticTokenChecker creates a static token auth checker.
func buildStaticTokenChecker(opts AuthOpts) (checker multiAuthChecker) {
	checker = func(r *http.Request) (username string) {
		username = TryStaticTokenAuth(r, resolveStaticToken(opts))
		return username
	}

	return checker
}

// buildOIDCChecker creates an OIDC auth checker.
func buildOIDCChecker(validator *OIDCValidator) (checker multiAuthChecker) {
	checker = func(r *http.Request) (username string) {
		username = TryOIDCAuth(r, validator)
		return username
	}

	return checker
}

// buildBasicHtpasswdChecker creates a basic htpasswd auth checker.
func buildBasicHtpasswdChecker(opts AuthOpts) (checker multiAuthChecker) {
	authenticator := auth.NewBasicAuthenticator("DBT Server", auth.HtpasswdFileProvider(opts.IdpFile))
	checker = func(r *http.Request) (username string) {
		username = authenticator.CheckAuth(r)
		if username != "" {
			log.Infof("Basic auth: user %s successfully authenticated (multi-auth)", username)
		}

		return username
	}

	return checker
}

// buildPubkeyFileChecker creates a pubkey-file auth checker.
func (d *DBTRepoServer) buildPubkeyFileChecker(opts AuthOpts, isPut bool) (checker multiAuthChecker, err error) {
	domain, domainErr := ExtractDomain(d.Address)
	if domainErr != nil {
		err = errors.Wrap(domainErr, "failed extracting domain for pubkey auth")
		return checker, err
	}

	pkFunc := pubkeyLookupFromFile(opts.IdpFile, isPut)
	checker = func(r *http.Request) (username string) {
		username = TryPubkeyAuth(r, domain, pkFunc)
		return username
	}

	return checker, err
}

// buildPubkeyFuncChecker creates a pubkey-func auth checker.
func (d *DBTRepoServer) buildPubkeyFuncChecker(opts AuthOpts) (checker multiAuthChecker, err error) {
	domain, domainErr := ExtractDomain(d.Address)
	if domainErr != nil {
		err = errors.Wrap(domainErr, "failed extracting domain for pubkey-func auth")
		return checker, err
	}

	idpFunc := opts.IdpFunc
	checker = func(r *http.Request) (username string) {
		pkFunc := func(subject string) (pubkeys []string, funcErr error) {
			pubkeys, funcErr = GetFuncUsername(idpFunc, subject)
			return pubkeys, funcErr
		}

		username = TryPubkeyAuth(r, domain, pkFunc)

		return username
	}

	return checker, err
}

// PutHandlerMultiAuth handles PUT requests with multiple authentication methods.
func (d *DBTRepoServer) PutHandlerMultiAuth(check multiAuthChecker) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		username := check(r)
		if username == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		putErr := d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
		if putErr != nil {
			wrappedErr := errors.Wrapf(putErr, "failed writing file %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			log.Error(wrappedErr)

			return
		}

		w.WriteHeader(http.StatusCreated)
	}

	return handler
}

// DeleteHandlerMultiAuth handles DELETE requests with multiple authentication methods.
func (d *DBTRepoServer) DeleteHandlerMultiAuth(check multiAuthChecker) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		username := check(r)
		if username == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		log.Infof("User %s authenticated for DELETE (multi-auth)", username)
		d.deleteAndRespond(w, r.URL.Path)
	}

	return handler
}

// CheckMultiAuthGet wraps a handler with multi-auth for GET requests.
func (d *DBTRepoServer) CheckMultiAuthGet(wrapped http.HandlerFunc, check multiAuthChecker) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		username := check(r)
		if username == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		r.Header.Set("X-Authenticated-Username", username)
		wrapped(w, r)
	}

	return handler
}
