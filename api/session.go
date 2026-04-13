package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

// serialCookie holds the minimal fields needed to reconstruct an http.Cookie
// for jar injection. Expiry is not persisted — the API server manages cookie
// lifetime.
type serialCookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
	Path   string `json:"path"`
}

// DefaultMaxWorkers is the default concurrency limit for session operations.
const DefaultMaxWorkers = 10

// DefaultThrottleBackoff is the initial backoff duration for rate limiting.
const DefaultThrottleBackoff = time.Second

// DefaultThrottleMaxDelay is the maximum backoff duration for rate limiting.
const DefaultThrottleMaxDelay = 30 * time.Second

// TokenWarnAge is the age at which session tokens are considered near expiry.
const TokenWarnAge = 20 * time.Hour

// TokenExpireAge is the age at which session tokens are considered expired.
const TokenExpireAge = 24 * time.Hour

// apiCookieURL returns the parsed Proton API base URL used for cookie scoping.
func apiCookieURL() *url.URL {
	u, _ := url.Parse(proton.DefaultHostURL)
	return u
}

// serializeCookies extracts cookies from the jar for the given API URL.
func serializeCookies(jar http.CookieJar, apiURL *url.URL) []serialCookie {
	cookies := jar.Cookies(apiURL)
	if len(cookies) == 0 {
		return nil
	}
	out := make([]serialCookie, len(cookies))
	for i, c := range cookies {
		out[i] = serialCookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		}
	}
	return out
}

// loadCookies injects persisted cookies into the jar for the given API URL.
func loadCookies(jar http.CookieJar, cookies []serialCookie, apiURL *url.URL) {
	if len(cookies) == 0 {
		return
	}
	httpCookies := make([]*http.Cookie, len(cookies))
	for i, c := range cookies {
		httpCookies[i] = &http.Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
		}
	}
	jar.SetCookies(apiURL, httpCookies)
}

// SessionOptions holds configuration for session creation.
type SessionOptions struct {
	MaxWorkers int
}

// Session holds an authenticated Proton API session with decrypted keyrings.
type Session struct {
	Client  *proton.Client
	Auth    proton.Auth
	BaseURL string // override for DoJSON; defaults to proton.DefaultHostURL
	manager *proton.Manager

	cookieJar http.CookieJar
	authMu    sync.Mutex // serializes auth handler updates

	// cachedAuthInfo holds the AuthInfo from the initial login attempt.
	// It is reused on HV retry so the SRP session matches the solved CAPTCHA.
	cachedAuthInfo *proton.AuthInfo

	MaxWorkers int
	Throttle   *Throttle

	addresses       map[string]proton.Address
	addressKeyRings map[string]*crypto.KeyRing

	user        proton.User
	UserKeyRing *crypto.KeyRing
}

// SessionFromCredentials initializes a new session from the provided config.
// The session is not fully usable until it has been Unlock'ed using the
// user-provided keypass.
func SessionFromCredentials(ctx context.Context, options []proton.Option, config *SessionConfig, managerHook func(*proton.Manager)) (*Session, error) {
	var err error

	if config.UID == "" {
		return nil, ErrMissingUID
	}

	if config.AccessToken == "" {
		return nil, ErrMissingAccessToken
	}

	if config.RefreshToken == "" {
		return nil, ErrMissingRefreshToken
	}

	var session Session
	session.MaxWorkers = DefaultMaxWorkers
	session.Throttle = NewThrottle(DefaultThrottleBackoff, DefaultThrottleMaxDelay)

	jar, _ := cookiejar.New(nil)
	session.cookieJar = jar

	slog.Debug("session.refresh client")

	session.manager = proton.New(append(options, proton.WithCookieJar(jar))...)

	if managerHook != nil {
		managerHook(session.manager)
	}

	slog.Debug("session.config", "uid", config.UID, "access_token", "<redacted>", "refresh_token", "<redacted>")
	session.Client = session.manager.NewClient(config.UID, config.AccessToken, config.RefreshToken)
	session.Auth = proton.Auth{
		UID:          config.UID,
		AccessToken:  config.AccessToken,
		RefreshToken: config.RefreshToken,
	}

	slog.Debug("session.GetUser")
	session.user, err = session.Client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// sessionFromLogin creates a session with common setup shared by
// SessionFromLogin and SessionFromLoginWithHV. It returns the prepared
// session and manager; the caller performs the actual login call.
func sessionFromLogin(options []proton.Option, managerHook func(*proton.Manager)) (*Session, *proton.Manager) {
	session := &Session{}
	session.MaxWorkers = DefaultMaxWorkers
	session.Throttle = NewThrottle(DefaultThrottleBackoff, DefaultThrottleMaxDelay)

	jar, _ := cookiejar.New(nil)
	session.cookieJar = jar

	session.manager = proton.New(append(options, proton.WithCookieJar(jar))...)

	if managerHook != nil {
		managerHook(session.manager)
	}

	return session, session.manager
}

// Unlock decrypts the user's account keyring and all address keyrings.
// The addresses slice is stored internally for backward compatibility with
// Drive methods that still reference s.addresses until they move to
// drive.Client.
func (s *Session) Unlock(keypass []byte, addresses []proton.Address) error {
	s.addresses = make(map[string]proton.Address, len(addresses))
	for _, addr := range addresses {
		s.addresses[addr.Email] = addr
	}

	var err error
	s.UserKeyRing, s.addressKeyRings, err = proton.Unlock(s.user, addresses, keypass, nil)
	return err
}

// AddressKeyRings returns the address keyrings produced by Unlock.
// Service-specific clients copy this map during their construction.
func (s *Session) AddressKeyRings() map[string]*crypto.KeyRing {
	return s.addressKeyRings
}

// User returns the proton.User for this session.
func (s *Session) User() proton.User { return s.user }

// Addresses fetches addresses from the API.
// Service-specific clients call this during their own construction.
func (s *Session) Addresses(ctx context.Context) ([]proton.Address, error) {
	return s.Client.GetAddresses(ctx)
}

// AddAuthHandler registers a handler for authentication events.
func (s *Session) AddAuthHandler(handler proton.AuthHandler) {
	s.Client.AddAuthHandler(handler)
}

// AddDeauthHandler registers a handler for deauthentication events.
func (s *Session) AddDeauthHandler(handler proton.Handler) {
	s.Client.AddDeauthHandler(handler)
}

// Stop closes the underlying API manager.
func (s *Session) Stop() {
	s.manager.Close()
}

// apiEnvelope is the standard Proton API response wrapper.
type apiEnvelope struct {
	Code  int    `json:"Code"`
	Error string `json:"Error,omitempty"`
}

// DoJSON executes an authenticated JSON API request against the Proton API.
// Method is "GET", "POST", "DELETE", etc. Path is relative to the API base
// (e.g. "/drive/shares/{id}/members"). If body is non-nil it is JSON-encoded
// as the request body. If result is non-nil the response body is JSON-decoded
// into it. Returns an *APIError on non-success API responses.
func (s *Session) DoJSON(ctx context.Context, method, path string, body, result any) error {
	reqURL := path
	if !strings.HasPrefix(path, "http") {
		base := s.BaseURL
		if base == "" {
			base = proton.DefaultHostURL
		}
		reqURL = base + path
	}

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("doJSON: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return fmt.Errorf("doJSON: new request: %w", err)
	}

	req.Header.Set("x-pm-uid", s.Auth.UID)
	req.Header.Set("Authorization", "Bearer "+s.Auth.AccessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Jar: s.cookieJar}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("doJSON: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("doJSON: read response: %w", err)
	}

	// Parse the envelope to check the API-level error code.
	var envelope apiEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("doJSON: unmarshal envelope: %w", err)
	}

	if envelope.Code != 1000 {
		return &APIError{
			Status:  resp.StatusCode,
			Code:    envelope.Code,
			Message: envelope.Error,
		}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("doJSON: unmarshal result: %w", err)
		}
	}

	return nil
}

// SessionRestore loads credentials from the store and creates an unlocked
// session. Returns ErrNotLoggedIn if no session is stored.
func SessionRestore(ctx context.Context, options []proton.Option, store SessionStore, managerHook func(*proton.Manager)) (*Session, error) {
	config, err := store.Load()
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, ErrNotLoggedIn
		}
		return nil, err
	}

	slog.Debug("SessionRestore", "uid", config.UID, "access_token", "<redacted>", "refresh_token", "<redacted>")

	// Staleness check.
	if !config.LastRefresh.IsZero() {
		age := time.Since(config.LastRefresh)
		if age > TokenExpireAge {
			slog.Warn("session tokens likely expired", "age", age)
		} else if age > TokenWarnAge {
			slog.Warn("session tokens near expiry", "age", age)
		}
	}

	session, err := SessionFromCredentials(ctx, options, config, managerHook)
	if err != nil {
		return nil, err
	}

	// Restore persisted cookies into the session's jar.
	loadCookies(session.cookieJar, config.Cookies, apiCookieURL())

	keypass, err := Base64Decode(config.SaltedKeyPass)
	if err != nil {
		return nil, err
	}

	addrs, err := session.Client.GetAddresses(ctx)
	if err != nil {
		return nil, err
	}

	if err := session.Unlock(keypass, addrs); err != nil {
		return nil, err
	}

	// Proactive refresh: make a lightweight API call to trigger
	// go-proton-api's auto-refresh if the access token is expired.
	if !config.LastRefresh.IsZero() && time.Since(config.LastRefresh) > TokenExpireAge {
		if _, err := session.Client.GetUser(ctx); err != nil {
			return nil, fmt.Errorf("proactive refresh: %w", err)
		}
	}

	return session, nil
}

// ReadySession restores a session from the store, registers auth/deauth
// handlers, and returns a fully initialized Session ready for use.
// This is the recommended entry point for consumers that need an
// authenticated session.
func ReadySession(ctx context.Context, options []proton.Option, store SessionStore, managerHook func(*proton.Manager)) (*Session, error) {
	session, err := SessionRestore(ctx, options, store, managerHook)
	if err != nil {
		return nil, err
	}
	session.AddAuthHandler(NewAuthHandler(store, session))
	session.AddDeauthHandler(NewDeauthHandler())
	return session, nil
}

// SessionSave persists session credentials, cookie jar state, and a refresh
// timestamp to the store.
func SessionSave(store SessionStore, session *Session, keypass []byte) error {
	apiURL := apiCookieURL()
	config := &SessionConfig{
		UID:           session.Auth.UID,
		AccessToken:   session.Auth.AccessToken,
		RefreshToken:  session.Auth.RefreshToken,
		SaltedKeyPass: Base64Encode(keypass),
		Cookies:       serializeCookies(session.cookieJar, apiURL),
		LastRefresh:   time.Now(),
	}
	return store.Save(config)
}

// SessionRevoke revokes the API session and deletes it from the store.
// If force is true, store deletion proceeds even when the API revoke fails.
func SessionRevoke(ctx context.Context, session *Session, store SessionStore, force bool) error {
	if session != nil {
		slog.Debug("SessionRevoke", "uid", session.Auth.UID)
		if err := session.Client.AuthRevoke(ctx, session.Auth.UID); err != nil {
			if !force {
				return err
			}
			slog.Error("SessionRevoke", "error", err)
		}
	}
	return store.Delete()
}

// SessionList returns account names from the session store.
func SessionList(store SessionStore) ([]string, error) {
	return store.List()
}

// SessionFromLogin initializes a new session from the provided login/password.
// If hvDetails is non-nil, the login includes the HV token for CAPTCHA retry.
// The same manager (and cookie jar) is used for both initial and HV-retried
// login attempts — this is required because Proton's backend correlates the
// solved CAPTCHA with the session cookie from the initial attempt.
//
// On error, the returned *Session is intentionally non-nil and reusable for
// SessionRetryWithHV. The manager and cookie jar must be preserved across
// attempts so that the solved CAPTCHA correlates with the session cookie
// established during the initial (failed) login request.
func SessionFromLogin(ctx context.Context, options []proton.Option, username string, password string, hvDetails *proton.APIHVDetails, managerHook func(*proton.Manager)) (*Session, error) {
	session, manager := sessionFromLogin(options, managerHook)

	slog.Debug("session.login", "username", username, "password", "<hidden>")

	// Fetch AuthInfo separately so we can cache it for HV retries.
	// The SRP session in AuthInfo is bound to the CAPTCHA token — reusing
	// it on retry is required for the solved token to be accepted.
	info, err := manager.AuthInfo(ctx, proton.AuthInfoReq{Username: username})
	if err != nil {
		return session, err
	}
	session.cachedAuthInfo = &info

	session.Client, session.Auth, err = manager.NewClientWithLoginWithCachedInfo(ctx, info, username, []byte(password), hvDetails)
	logCookies("session.login.done", session)
	slog.Debug("session.login.done", "error", err)
	if err != nil {
		return session, err
	}

	return session, nil
}

// SessionRetryWithHV retries login on an existing session (reusing its
// manager and cookie jar) with HV details after the user solved the CAPTCHA.
// A fresh AuthInfo is fetched because the original SRP session is invalidated
// by the 9001 response. The solved CAPTCHA composite token is NOT bound to
// the SRP session — it's bound to the HumanVerificationToken.
func SessionRetryWithHV(ctx context.Context, session *Session, username, password string, hv *proton.APIHVDetails) error {
	logCookies("session.login.hv.before", session)
	slog.Debug("session.login.hv", "username", username, "password", "<hidden>")

	var err error
	session.Client, session.Auth, err = session.manager.NewClientWithLoginWithHVToken(ctx, username, []byte(password), hv)
	logCookies("session.login.hv.after", session)
	return err
}

// logCookies logs the current cookie names in the session's jar for debugging.
// Only names are logged — values are sensitive and must not appear in logs.
func logCookies(label string, session *Session) {
	apiURL := apiCookieURL()
	cookies := session.cookieJar.Cookies(apiURL)
	names := make([]string, len(cookies))
	for i, c := range cookies {
		names[i] = c.Name
	}
	slog.Debug(label, "cookies", names)
}
