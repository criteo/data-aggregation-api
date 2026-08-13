package auth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/go-ldap/ldap/v3"
)

type connectionStatus bool

var ErrLDAPTimeout = errors.New("LDAP timeout")

const (
	connectionClosed connectionStatus = false
	connectionUp     connectionStatus = true
)

type authRequest struct {
	authResp chan bool
	username string
	password string
}

type result struct {
	auth bool
	conn connectionStatus
}

type LDAPAuth struct {
	tlsConfig             *tls.Config
	reqCh                 chan authRequest
	ldapURL               string
	bindDN                string
	password              string
	baseDN                string
	maxConnectionLifetime time.Duration
}

func NewLDAPAuth(ldapURL string, bindDN string, password string, baseDN string, tlsConfig *tls.Config) *LDAPAuth {
	return &LDAPAuth{
		tlsConfig: tlsConfig,
		ldapURL:   ldapURL,
		bindDN:    bindDN,
		password:  password,
		baseDN:    baseDN,
		reqCh:     make(chan authRequest),
	}
}

func SetLDAPDefaultTimeout(timeout time.Duration) {
	ldap.DefaultTimeout = timeout
}

// SetMaxConnectionLifetime sets the maximum lifetime of a connection.
//
// The maximum lifetime is the maximum amount of time a connection may be reused for.
// This is not a guarantee, as the connection may have been closed by the server before reaching that timer.
func (l *LDAPAuth) SetMaxConnectionLifetime(maxConnectionLifetime time.Duration) {
	l.maxConnectionLifetime = maxConnectionLifetime
}

// StartAuthenticationWorkers starts a pool of workers that will handle the authentication requests.
func (l *LDAPAuth) StartAuthenticationWorkers(ctx context.Context, workersCount int) error {
	if workersCount <= 0 {
		return fmt.Errorf("'WorkersCount' must be greater than 0: %d", workersCount)
	}
	for range workersCount {
		go l.spawnConnectionWorker(ctx)
	}
	return nil
}

func (l *LDAPAuth) AuthenticateUser(username string, password string) bool {
	req := authRequest{
		username: username,
		password: password,
		authResp: make(chan bool),
	}
	l.reqCh <- req
	return <-req.authResp
}

func (l *LDAPAuth) spawnConnectionWorker(ctx context.Context) {
	const maxAttempts = 3
	var conn *ldap.Conn
	var err error
	tick := time.NewTicker(l.maxConnectionLifetime)

	for {
		select {
		case req := <-l.reqCh:
			auth := false
			attempt := 1
			for attempt <= maxAttempts {
				attempt++
				log.Debug().Msgf("worker LDAP authentication attempt number %d", attempt)
				// (re)connect if needed
				if conn == nil || conn.IsClosing() {
					log.Debug().Msg("LDAP connection is closed, reconnecting")
					conn, err = l.connect()
					if err != nil {
						log.Error().Err(err).Msg("worker LDAP reconnection failed")
						break
					}
				}

				// bind with the user credentials
				var connState connectionStatus
				func() {
					// this anonymous function ensures the context is released as soon as possible (because of the for loop)
					ctxTimeout, cancel := context.WithTimeoutCause(ctx, ldap.DefaultTimeout, ErrLDAPTimeout)
					defer cancel()
					auth, connState = l.authenticateWithTimeout(ctxTimeout, conn, req.username, req.password)
				}()

				if connState == connectionClosed {
					log.Debug().Msg("LDAP connection was closed by the server, closing on client side")
					if err := conn.Close(); err != nil {
						log.Error().Err(err).Msg("connection was closed by the server but failed to close on client side")
					}
				} else {
					// LDAP connection is still up, we accept the authentication result
					log.Debug().Msg("auth response valid")
					break
				}
			}

			log.Debug().Msgf("worker LDAP authentication attempt number %d, result: %t", attempt, auth)

			req.authResp <- auth
			tick.Reset(l.maxConnectionLifetime)

		case <-tick.C:
			// close connection if no request has been made
			log.Debug().Msg("timer reached, closing connection")
			closeLDAPConnection(conn)

		case <-ctx.Done():
			// gracefully close connection if context is done
			log.Debug().Msg("context is closed, closing connection")
			closeLDAPConnection(conn)
			return
		}
	}
}

func closeLDAPConnection(conn *ldap.Conn) {
	if conn != nil && !conn.IsClosing() {
		if err := conn.Close(); err != nil {
			log.Error().Err(err).Msg("unable to close the LDAP connection")
		}
	}
}

// authenticateWithTimeout performs the authentication against LDAP with a timeout.
func (l *LDAPAuth) authenticateWithTimeout(ctx context.Context, conn *ldap.Conn, username, password string) (bool, connectionStatus) {
	// request the authentication
	res := make(chan result, 1)
	go func() {
		a, c := l.authenticate(conn, username, password)
		res <- result{auth: a, conn: c}
	}()

	var connState connectionStatus
	var auth bool

	// handle timeout and context closing
	select {
	case r := <-res:
		auth = r.auth
		connState = r.conn
	case <-ctx.Done():
		auth = false
		if errors.Is(context.Cause(ctx), ErrLDAPTimeout) {
			log.Error().Msg("LDAP authentication timeout")
			closeLDAPConnection(conn)
		} else {
			connState = connectionClosed
		}
	}
	return auth, connState
}

func (l *LDAPAuth) connect() (*ldap.Conn, error) {
	conn, err := ldap.DialURL(l.ldapURL, ldap.DialWithTLSConfig(l.tlsConfig))
	if err != nil {
		return nil, fmt.Errorf("failed to dial to the LDAP server: %w", err)
	}

	return conn, nil
}

// authenticate performs the authentication against LDAP.
// The first returned boolean is true if the authentication is successful, false otherwise.
// The second returned boolean is false if the connection is closed, true otherwise.
func (l *LDAPAuth) authenticate(conn *ldap.Conn, username string, password string) (bool, connectionStatus) {
	if err := conn.Bind(l.bindDN, l.password); err != nil {
		log.Error().Err(err).Str("bindDN", l.bindDN).Msg("failed to bind to LDAP")

		// detect TCP connection closed or any network errors
		if ldap.IsErrorWithCode(err, ldap.ErrorNetwork) {
			return false, connectionClosed
		}
		return false, connectionUp
	}
	search, err := conn.Search(ldap.NewSearchRequest(
		l.baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		fmt.Sprintf("(&(objectClass=user)(sAMAccountName=%s))", username),
		[]string{"dn"},
		nil,
	))

	const userKey = "user"
	if err != nil {
		log.Error().Err(err).Str(userKey, username).Msg("failed to perform LDAP search to find user")
		return false, connectionUp
	}
	if len(search.Entries) != 1 {
		log.Error().Str(userKey, username).Msg("no result or more than 1 result found for user")
		return false, connectionUp
	}
	if err := conn.Bind(search.Entries[0].DN, password); err != nil {
		log.Error().Err(err).Str(userKey, username).Msg("failed to bind with user")
		return false, connectionUp
	}
	return true, connectionUp
}
