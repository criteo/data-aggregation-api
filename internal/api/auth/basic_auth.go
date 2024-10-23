package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"

	"github.com/criteo/data-aggregation-api/internal/config"
	"github.com/rs/zerolog/log"
)

const (
	unauthorizedResponse = `{"auth": "unauthorized"}`
	wwwAuthenticate      = "WWW-Authenticate"
	realm                = `Basic realm="restricted"`
)

type authMode string

const (
	noAuth   authMode = "None"
	ldapMode authMode = "LDAP"
)

type BasicAuth struct {
	ldapAuth *LDAPAuth
	mode     authMode
}

func NewBasicAuth(ctx context.Context, cfg config.AuthConfig) (BasicAuth, error) {
	b := BasicAuth{mode: noAuth}

	if cfg.LDAP == nil {
		return b, nil
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.LDAP.InsecureSkipVerify, //nolint:gosec // configurable on purpose
	}
	ldap := NewLDAPAuth(cfg.LDAP.URL, cfg.LDAP.BindDN, cfg.LDAP.Password, cfg.LDAP.BaseDN, tlsConfig)
	if err := b.configureLdap(ldap); err != nil {
		return b, fmt.Errorf("failed to configure the request authenticator: %w", err)
	}
	ldap.SetMaxConnectionLifetime(cfg.LDAP.MaxConnectionLifetime)
	b.mode = ldapMode

	// trying a connection to LDAP to check the configuration
	conn, err := ldap.connect()
	if err != nil {
		log.Warn().Err(err).Msg("test connection to LDAP failed")
	}
	if conn != nil {
		if err := conn.Close(); err != nil {
			log.Warn().Err(err).Msg("failed to close LDAP test connection")
		}
	}

	if err := ldap.StartAuthenticationWorkers(ctx, cfg.LDAP.WorkersCount); err != nil {
		return b, fmt.Errorf("failed to start LDAP workers: %w", err)
	}

	return b, nil
}

func (b *BasicAuth) configureLdap(ldap *LDAPAuth) error {
	if ldap == nil {
		return errors.New("LDAP configuration is missing")
	}
	b.ldapAuth = ldap

	return nil
}

func (b *BasicAuth) Wrap(next http.HandlerFunc) http.HandlerFunc {
	switch b.mode {
	case noAuth:
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next(w, r) })
	case ldapMode:
		return BasicAuthLDAP(b.ldapAuth, next)
	default:
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			log.Error().Str("auth-method", string(b.mode)).Str("authentication issue", "bad server configuration").Send()
			http.Error(w, "authentication issue: bad server configuration", http.StatusInternalServerError)
		})
	}
}

// BasicAuthLDAP is a middleware wrapping the target HTTP HandlerFunc.
// It retrieves BasicAuth credentials and authenticate against LDAP.
func BasicAuthLDAP(ldapAuth *LDAPAuth, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok {
			w.Header().Set(wwwAuthenticate, realm)
			http.Error(w, unauthorizedResponse, http.StatusUnauthorized)
			return
		}
		if ok := ldapAuth.AuthenticateUser(username, password); !ok {
			w.Header().Set(wwwAuthenticate, realm)
			http.Error(w, unauthorizedResponse, http.StatusUnauthorized)
			return
		}
		next(w, r)
	})
}
