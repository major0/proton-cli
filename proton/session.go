package proton

import (
	"context"
	"errors"
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
	manager *proton.Manager

	cookieJar http.CookieJar

	// cachedAuthInfo holds the AuthInfo from the initial login attempt.
	// It is reused on HV retry so the SRP session matches the solved CAPTCHA.
	cachedAuthInfo *proton.AuthInfo

	MaxWorkers int

	addresses      map[string]proton.Address
	AddressKeyRing map[string]*crypto.KeyRing

	user        proton.User
	UserKeyRing *crypto.KeyRing
}

// SessionFromCredentials initializes a new session from the provided credentials.
// The session is not fully usable until it has been Unlock'ed using the
// user-provided keypass.
func SessionFromCredentials(ctx context.Context, options []proton.Option, creds *SessionCredentials, managerHook func(*proton.Manager)) (*Session, error) {
	var err error

	// Initialize the client from our cached credentials
	if creds.UID == "" {
		return nil, ErrMissingUID
	}

	if creds.AccessToken == "" {
		return nil, ErrMissingAccessToken
	}

	if creds.RefreshToken == "" {
		return nil, ErrMissingRefreshToken
	}

	var session Session
	session.MaxWorkers = 10

	jar, _ := cookiejar.New(nil)
	session.cookieJar = jar

	slog.Debug("session.refresh client")

	session.manager = proton.New(append(options, proton.WithCookieJar(jar))...)

	if managerHook != nil {
		managerHook(session.manager)
	}

	slog.Debug("session.config", "uid", creds.UID, "access_token", creds.AccessToken, "refresh_token", creds.RefreshToken)
	session.Client = session.manager.NewClient(creds.UID, creds.AccessToken, creds.RefreshToken)
	session.Auth = proton.Auth{
		UID:          creds.UID,
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
	}

	slog.Debug("session.GetUser")
	session.user, err = session.Client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("session.GetAddresses")

	addrs, err := session.Client.GetAddresses(ctx)
	if err != nil {
		return nil, err
	}

	session.addresses = make(map[string]proton.Address)
	for _, addr := range addrs {
		session.addresses[addr.Email] = addr
	}

	return &session, nil
}

// sessionFromLogin creates a session with common setup shared by
// SessionFromLogin and SessionFromLoginWithHV. It returns the prepared
// session and manager; the caller performs the actual login call.
func sessionFromLogin(options []proton.Option, managerHook func(*proton.Manager)) (*Session, *proton.Manager) {
	session := &Session{}
	session.MaxWorkers = 10

	jar, _ := cookiejar.New(nil)
	session.cookieJar = jar

	session.manager = proton.New(append(options, proton.WithCookieJar(jar))...)

	if managerHook != nil {
		managerHook(session.manager)
	}

	return session, session.manager
}

/* Unlock the user's account keyring, as well as all keyring's associated
 * with alternate addresses. */
func (s *Session) Unlock(keypass string) error {
	var err error

	var addresses []proton.Address
	for _, addr := range s.addresses {
		addresses = append(addresses, addr)
	}

	s.UserKeyRing, s.AddressKeyRing, err = proton.Unlock(s.user, addresses, []byte(keypass), nil)
	if err != nil {
		return err
	}

	return nil
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

// ListVolumes returns all volumes accessible by this session.
func (s *Session) ListVolumes(ctx context.Context) ([]Volume, error) {
	pVolumes, err := s.Client.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	volumes := make([]Volume, len(pVolumes))
	for i := range pVolumes {
		volumes[i] = Volume{pVolume: pVolumes[i], session: s}
	}

	return volumes, nil
}

// GetVolume returns the volume with the given ID.
func (s *Session) GetVolume(ctx context.Context, id string) (Volume, error) {
	pVolume, err := s.Client.GetVolume(ctx, id)
	if err != nil {
		return Volume{}, err
	}

	return Volume{pVolume: pVolume, session: s}, nil
}

// ListSharesMetadata returns metadata for all shares visible to this session.
func (s *Session) ListSharesMetadata(ctx context.Context, all bool) ([]ShareMetadata, error) {
	pShares, err := s.Client.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	shares := make([]ShareMetadata, len(pShares))
	for i := range pShares {
		shares[i] = ShareMetadata(pShares[i])
	}
	return shares, nil
}

// GetShareMetadata returns the metadata for the share with the given ID.
func (s *Session) GetShareMetadata(ctx context.Context, id string) (ShareMetadata, error) {
	shares, err := s.Client.ListShares(ctx, true)
	if err != nil {
		return ShareMetadata{}, err
	}

	for _, share := range shares {
		if share.ShareID == id {
			return ShareMetadata(share), nil
		}
	}

	return ShareMetadata{}, nil
}

// ListShares returns all fully-resolved shares visible to this session.
func (s *Session) ListShares(ctx context.Context, all bool) ([]Share, error) {
	return s.listShares(ctx, "", all)
}

func (s *Session) listShares(ctx context.Context, volumeID string, all bool) ([]Share, error) {
	pshares, err := s.Client.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	slog.Debug("session.ListShares", "shares", len(pshares))
	slog.Debug("session.ListShares", "volumID", volumeID)

	var wg sync.WaitGroup
	idQueue := make(chan string)
	shareQueue := make(chan *Share)
	for i := 0; i < min(s.MaxWorkers, len(pshares)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range idQueue {
				share, err := s.GetShare(ctx, id)
				if err != nil {
					slog.Error("worker", "shareID", id, "error", err)
					continue
				}
				shareQueue <- share
			}
		}()
	}

	// Spawn a producer to feed the idQueue, respecting cancellation.
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(idQueue)
		for _, s := range pshares {
			if volumeID != "" && volumeID != s.VolumeID {
				continue
			}
			select {
			case idQueue <- s.ShareID:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to finish, then close the shareQueue to
	// signal the main goroutine.
	go func() {
		wg.Wait()
		close(shareQueue)
	}()

	var shares []Share
	for share := range shareQueue {
		shares = append(shares, *share)
	}

	return shares, nil
}

// GetShare returns the fully-resolved share with the given ID.
func (s *Session) GetShare(ctx context.Context, id string) (*Share, error) {
	pShare, err := s.Client.GetShare(ctx, id)
	if err != nil {
		return nil, err
	}

	shareAddrKR := s.AddressKeyRing[pShare.AddressID]
	shareKR, err := pShare.GetKeyRing(shareAddrKR)
	if err != nil {
		return nil, err
	}

	share := Share{
		keyRing:     shareKR,
		protonShare: &pShare,
		session:     s,
	}

	pLink, err := s.Client.GetLink(ctx, pShare.ShareID, pShare.LinkID)
	if err != nil {
		return nil, err
	}

	link := s.newLink(ctx, &share, nil, &pLink)
	share.Link = link

	return &share, nil
}

// ResolveShare finds a share by its root link name.
func (s *Session) ResolveShare(ctx context.Context, name string, all bool) (*Share, error) {
	shares, err := s.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	for i := range shares {
		shareName, err := shares[i].Link.Name()
		if err != nil {
			continue
		}
		if shareName == name {
			return &shares[i], nil
		}
	}

	return nil, ErrFileNotFound
}

// ResolvePath resolves a slash-separated path to a link across all shares.
func (s *Session) ResolvePath(ctx context.Context, path string, all bool) (*Link, error) {
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return nil, ErrInvalidPath
	}

	share, err := s.ResolveShare(ctx, parts[0], all)
	if err != nil {
		return nil, err
	}

	return share.Link.resolveParts(ctx, parts[1:])
}

func (s *Session) newLink(_ context.Context, share *Share, parent *Link, pLink *proton.Link) *Link {
	return newLink(pLink, parent, share, s)
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

	slog.Debug("SessionRestore", "uid", config.UID, "access_token", config.AccessToken, "refresh_token", config.RefreshToken)

	creds := &SessionCredentials{
		UID:          config.UID,
		AccessToken:  config.AccessToken,
		RefreshToken: config.RefreshToken,
	}

	session, err := SessionFromCredentials(ctx, options, creds, managerHook)
	if err != nil {
		return nil, err
	}

	// Restore persisted cookies into the session's jar.
	loadCookies(session.cookieJar, config.Cookies, apiCookieURL())

	keypass, err := Base64Decode(config.SaltedKeyPass)
	if err != nil {
		return nil, err
	}

	if err := session.Unlock(string(keypass)); err != nil {
		return nil, err
	}

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

// logCookies logs the current cookies in the session's jar for debugging.
func logCookies(label string, session *Session) {
	apiURL := apiCookieURL()
	cookies := session.cookieJar.Cookies(apiURL)
	names := make([]string, len(cookies))
	for i, c := range cookies {
		names[i] = c.Name + "=" + c.Value
	}
	slog.Debug(label, "cookies", names)
}
