// Copyright © 2025 Nik Ogura <nik.ogura@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbt

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

//nolint:revive,staticcheck // AUTH_OIDC is a public API constant
const AUTH_OIDC = "oidc"

// DefaultUsernameClaim is the default OIDC claim to extract username from.
const DefaultUsernameClaim = "sub"

// OIDCAuthOpts holds configuration for OIDC authentication.
type OIDCAuthOpts struct {
	IssuerURL        string            `json:"issuerUrl"`                  // OIDC issuer URL (e.g., https://dex.example.com)
	Audiences        []string          `json:"audiences"`                  // Expected audience claims
	UsernameClaimKey string            `json:"usernameClaimKey,omitempty"` // Claim to extract username from (default: "sub")
	RequiredClaims   map[string]string `json:"requiredClaims,omitempty"`   // Additional claims that must match
	AllowedGroups    []string          `json:"allowedGroups,omitempty"`    // Groups authorized to access (empty = all authenticated users)
	SkipIssuerVerify bool              `json:"skipIssuerVerify,omitempty"` // Skip issuer verification (testing only)
	JWKSCacheSeconds int               `json:"jwksCacheSeconds,omitempty"` // JWKS cache duration (default: 300)
}

// OIDCValidator validates OIDC tokens from an identity provider.
type OIDCValidator struct {
	config   *OIDCAuthOpts
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
}

// OIDCClaims represents the user claims extracted from an OIDC token.
type OIDCClaims struct {
	Subject  string   `json:"sub"`
	Email    string   `json:"email,omitempty"`
	Username string   `json:"preferred_username,omitempty"`
	Name     string   `json:"name,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// NewOIDCValidator creates a new OIDC validator with the given configuration.
func NewOIDCValidator(ctx context.Context, config *OIDCAuthOpts) (validator *OIDCValidator, err error) {
	if config.IssuerURL == "" {
		err = errors.New("OIDC issuer URL is required")
		return validator, err
	}

	if len(config.Audiences) == 0 {
		err = errors.New("at least one OIDC audience is required")
		return validator, err
	}

	// Set defaults
	if config.UsernameClaimKey == "" {
		config.UsernameClaimKey = DefaultUsernameClaim
	}

	if config.JWKSCacheSeconds == 0 {
		config.JWKSCacheSeconds = 300 // 5 minutes
	}

	// Create OIDC provider (this fetches the .well-known/openid-configuration)
	provider, providerErr := oidc.NewProvider(ctx, config.IssuerURL)
	if providerErr != nil {
		err = errors.Wrapf(providerErr, "failed to create OIDC provider for %s", config.IssuerURL)
		return validator, err
	}

	// Configure the verifier
	verifierConfig := &oidc.Config{
		ClientID:          config.Audiences[0], // Primary audience
		SkipIssuerCheck:   config.SkipIssuerVerify,
		SkipClientIDCheck: len(config.Audiences) > 1, // We'll check audiences manually if multiple
	}

	verifier := provider.Verifier(verifierConfig)

	validator = &OIDCValidator{
		config:   config,
		provider: provider,
		verifier: verifier,
	}

	log.Infof("OIDC validator initialized for issuer: %s", config.IssuerURL)

	return validator, err
}

// ValidateToken validates an OIDC token and returns the extracted claims.
func (v *OIDCValidator) ValidateToken(ctx context.Context, tokenString string) (claims *OIDCClaims, err error) {
	// Verify the token
	idToken, verifyErr := v.verifier.Verify(ctx, tokenString)
	if verifyErr != nil {
		err = errors.Wrap(verifyErr, "failed to verify OIDC token")
		return claims, err
	}

	// Extract standard claims
	claims = &OIDCClaims{}
	err = idToken.Claims(claims)
	if err != nil {
		err = errors.Wrap(err, "failed to extract claims from OIDC token")
		return claims, err
	}

	// Validate audience if multiple are configured
	if len(v.config.Audiences) > 1 {
		if !v.audienceMatches(idToken.Audience) {
			err = errors.Errorf("token audience %v does not match expected audiences %v",
				idToken.Audience, v.config.Audiences)
			return claims, err
		}
	}

	// Validate required claims if configured
	if len(v.config.RequiredClaims) > 0 {
		err = v.validateRequiredClaims(idToken)
		if err != nil {
			return claims, err
		}
	}

	// Validate group membership if configured
	if len(v.config.AllowedGroups) > 0 {
		err = v.validateGroupMembership(claims.Groups)
		if err != nil {
			return claims, err
		}
	}

	return claims, err
}

// GetUsername returns the username from claims based on configuration.
func (v *OIDCValidator) GetUsername(claims *OIDCClaims) (username string) {
	switch v.config.UsernameClaimKey {
	case "email":
		username = claims.Email
	case "preferred_username":
		username = claims.Username
	case "name":
		username = claims.Name
	case DefaultUsernameClaim:
		username = claims.Subject
	default:
		// Default to subject
		username = claims.Subject
	}

	// Fallback chain if preferred claim is empty
	if username == "" {
		if claims.Username != "" {
			username = claims.Username
		} else if claims.Name != "" {
			username = claims.Name
		} else if claims.Email != "" {
			username = claims.Email
		} else {
			username = claims.Subject
		}
	}

	return username
}

func (v *OIDCValidator) audienceMatches(tokenAudiences []string) (matches bool) {
	for _, tokenAud := range tokenAudiences {
		for _, expectedAud := range v.config.Audiences {
			if tokenAud == expectedAud {
				matches = true
				return matches
			}
		}
	}
	matches = false
	return matches
}

func (v *OIDCValidator) validateRequiredClaims(idToken *oidc.IDToken) (err error) {
	// Extract all claims as a map
	var allClaims map[string]interface{}
	err = idToken.Claims(&allClaims)
	if err != nil {
		err = errors.Wrap(err, "failed to extract all claims for validation")
		return err
	}

	for key, expectedValue := range v.config.RequiredClaims {
		actualValue, exists := allClaims[key]
		if !exists {
			err = errors.Errorf("required claim %q is missing", key)
			return err
		}

		actualStr, ok := actualValue.(string)
		if !ok {
			err = errors.Errorf("required claim %q is not a string", key)
			return err
		}

		if actualStr != expectedValue {
			err = errors.Errorf("required claim %q has value %q, expected %q", key, actualStr, expectedValue)
			return err
		}
	}

	return err
}

func (v *OIDCValidator) validateGroupMembership(userGroups []string) (err error) {
	// If no groups are configured, allow all authenticated users
	if len(v.config.AllowedGroups) == 0 {
		return err
	}

	for _, userGroup := range userGroups {
		for _, allowedGroup := range v.config.AllowedGroups {
			if userGroup == allowedGroup {
				return err
			}
		}
	}

	err = errors.Errorf("user not in any allowed groups %v, user groups: %v",
		v.config.AllowedGroups, userGroups)
	return err
}

// TryOIDCAuth attempts OIDC token authentication without writing to the response.
// Returns the username if authenticated, empty string otherwise.
func TryOIDCAuth(r *http.Request, validator *OIDCValidator) (username string) {
	if validator == nil {
		return username
	}

	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return username
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" {
		return username
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	claims, validateErr := validator.ValidateToken(ctx, tokenString)
	if validateErr != nil {
		return username
	}

	username = validator.GetUsername(claims)
	if username == "" {
		return username
	}

	log.Infof("OIDC: Subject %s successfully authenticated (multi-auth)", username)

	return username
}

// CheckOIDCAuth validates the OIDC token from the Authorization header.
// Returns the username if valid, empty string if invalid.
func CheckOIDCAuth(w http.ResponseWriter, r *http.Request, validator *OIDCValidator) (username string) {
	// Extract token from Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Info("OIDC Auth Failed: no Authorization header")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	// Expect "Bearer <token>" format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		log.Info("OIDC Auth Failed: Authorization header is not Bearer token")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" {
		log.Info("OIDC Auth Failed: empty Bearer token")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	// Create context with timeout for validation
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Validate the token
	claims, err := validator.ValidateToken(ctx, tokenString)
	if err != nil {
		log.Errorf("OIDC Auth Failed: %s", err)
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	// Extract username from claims
	username = validator.GetUsername(claims)
	if username == "" {
		log.Error("OIDC Auth Failed: no username in claims")
		w.WriteHeader(http.StatusUnauthorized)
		return username
	}

	log.Infof("OIDC: Subject %s successfully authenticated", username)

	return username
}

// WrapOIDC wraps an AuthenticatedHandlerFunc with OIDC authentication.
func WrapOIDC(wrapped AuthenticatedHandlerFunc, validator *OIDCValidator) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		if username := CheckOIDCAuth(w, r, validator); username != "" {
			ar := &AuthenticatedRequest{Request: *r, Username: username}
			wrapped(w, ar)
		}
	}
	return handler
}

// CheckOIDCGet wraps a handler function with OIDC authentication for GET requests.
func (d *DBTRepoServer) CheckOIDCGet(wrapped http.HandlerFunc, validator *OIDCValidator) (handler http.HandlerFunc) {
	handler = WrapOIDC(func(w http.ResponseWriter, ar *AuthenticatedRequest) {
		ar.Header.Set("X-Authenticated-Username", ar.Username)
		wrapped(w, &ar.Request)
	}, validator)
	return handler
}

// PutHandlerOIDC handles PUT requests with OIDC authentication.
func (d *DBTRepoServer) PutHandlerOIDC(validator *OIDCValidator) (handler http.HandlerFunc) {
	handler = func(w http.ResponseWriter, r *http.Request) {
		username := CheckOIDCAuth(w, r, validator)
		if username == "" {
			return // Auth failed, response already sent
		}

		err := d.HandlePut(r.URL.Path, r.Body, r.Header.Get("X-Checksum-Md5"), r.Header.Get("X-Checksum-Sha1"), r.Header.Get("X-Checksum-Sha256"))
		if err != nil {
			errWrapped := errors.Wrapf(err, "failed writing file %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			log.Error(errWrapped)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
	return handler
}
