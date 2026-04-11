package proton

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type SessionOptions struct {
	MaxWorkers int
}

type Session struct {
	Client  *proton.Client
	Auth    proton.Auth
	manager *proton.Manager

	cookieJar http.CookieJar

	MaxWorkers int

	addresses      map[string]proton.Address
	AddressKeyRing map[string]*crypto.KeyRing

	user        proton.User
	UserKeyRing *crypto.KeyRing
}

/* Initialize a new session frmo the provided credentials. The session is
 * not fully usable until it has been Unlock()'ed using the user-provided
 * keypass */
func SessionFromCredentials(ctx context.Context, options []proton.Option, creds *SessionCredentials, managerHook func(*proton.Manager)) (*Session, error) {
	var err error

	// Initialize the client from our cahced credentials
	if creds.UID == "" {
		return nil, ErrorMissingUID
	}

	if creds.AccessToken == "" {
		return nil, ErrorMissingAccessToken
	}

	if creds.RefreshToken == "" {
		return nil, ErrorMissingRefreshToken
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

// SessionFromLogin initializes a new session from the provided login/password.
// The returned session may have extra authentication requirements, such as 2FA.
// Once all authentication challenges have been met, the session will still
// need to be Unlock()'ed to gain access to the User and Address keyrings.
func SessionFromLogin(ctx context.Context, options []proton.Option, username string, password string, managerHook func(*proton.Manager)) (*Session, error) {
	session, manager := sessionFromLogin(options, managerHook)

	slog.Debug("session.login", "username", username, "password", "<hidden>")

	var err error
	session.Client, session.Auth, err = manager.NewClientWithLogin(ctx, username, []byte(password))
	if err != nil {
		return nil, err
	}

	return session, nil
}

// SessionFromLoginWithHV performs SRP login with a pre-solved HV token.
// It shares common setup with SessionFromLogin but calls
// NewClientWithLoginWithHVToken instead of NewClientWithLogin.
func SessionFromLoginWithHV(ctx context.Context, options []proton.Option, username, password string, hv *proton.APIHVDetails, managerHook func(*proton.Manager)) (*Session, error) {
	session, manager := sessionFromLogin(options, managerHook)

	slog.Debug("session.login.hv", "username", username, "password", "<hidden>")

	var err error
	session.Client, session.Auth, err = manager.NewClientWithLoginWithHVToken(ctx, username, []byte(password), hv)
	if err != nil {
		return nil, err
	}

	return session, nil
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

func (s *Session) AddAuthHandler(handler proton.AuthHandler) {
	s.Client.AddAuthHandler(handler)
}

func (s *Session) AddDeauthHandler(handler proton.Handler) {
	s.Client.AddDeauthHandler(handler)
}

func (s *Session) Stop() {
	s.manager.Close()
}

func (s *Session) ListVolumes(ctx context.Context) ([]Volume, error) {
	pVolumes, err := s.Client.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	volumes := make([]Volume, 0, len(pVolumes))
	for i := range pVolumes {
		volumes[i] = Volume{pVolume: pVolumes[i], session: s}
	}

	return volumes, nil
}

func (s *Session) GetVolume(ctx context.Context, id string) (Volume, error) {
	pVolume, err := s.Client.GetVolume(ctx, id)
	if err != nil {
		return Volume{}, err
	}

	return Volume{pVolume: pVolume, session: s}, nil
}

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
			//slog.Debug("starting worker", "id", i)
			defer wg.Done()
			for {
				id, ok := <-idQueue
				if !ok {
					//slog.Debug("ending worker", "id", i)
					return
				}

				//slog.Debug("worker", "operation", "get", "ShareID", id)
				share, err := s.GetShare(ctx, id)
				if err != nil {
					slog.Error("worker", "shareID", id, "error", err)
					continue
				}
				shareQueue <- share
				//slog.Debug("worker", "operation", "got", "id", id)
			}
		}()
	}

	/* Spawn a producer to feed the idQueue as fast as the workers can
	 * consume it. */
	wg.Add(1)
	go func() {
		//slog.Debug("starting producer")
		defer wg.Done()
		for _, s := range pshares {
			if volumeID != "" && volumeID != s.VolumeID {
				//slog.Debug("producer", "operation", "skip", "id", s.ShareID)
				continue
			}
			//slog.Debug("producer", "operation", "put", "id", s.ShareID)
			idQueue <- s.ShareID
		}
		/* Let the workers know there is nothing more to be written to the
		 * queue */
		close(idQueue)
		//slog.Debug("ending producer")
	}()

	/* Spawn a go routine that waits for all the workers to be
	 * finished and then closes the shareQueue. This acts to signal
	 * the main thread that all the workers are done. Until then the
	 * main thread can `select` the shareQueue appending shares to an
	 * array. */
	go func() {
		//slog.Debug("session.ListShares", "sync", "wait")
		wg.Wait()
		close(shareQueue)
		//slog.Debug("session.ListShares", "sync", "done")
	}()

	var shares []Share
	for {
		share, ok := <-shareQueue
		if !ok {
			for s := range shareQueue {
				//slog.Debug("session.ListShares", "share", s.protonShare.ShareID)
				shares = append(shares, *s)
			}
			break
		}
		//slog.Debug("session.ListShares", "share", share.protonShare.ShareID)
		shares = append(shares, *share)
	}

	//slog.Debug("session.ListShares", "workers", "done")

	return shares, nil
}

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

	link, err := s.newLink(ctx, &share, nil, &pLink)
	if err != nil {
		return nil, err
	}

	share.Link = link

	return &share, nil
}

func (s *Session) ResolveShare(ctx context.Context, name string, all bool) (*Share, error) {
	shares, err := s.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	for _, share := range shares {
		if share.Link.Name == name {
			return &share, nil
		}
	}

	return nil, ErrFileNotFound
}

func (s *Session) ResolvePath(ctx context.Context, path string, all bool) (*Link, error) {
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		return nil, ErrInvalidPath
	}

	share, err := s.ResolveShare(ctx, parts[0], all)
	if err != nil {
		return nil, err
	}

	link, err := share.Link.resolveParts(ctx, parts[1:], all)
	if err != nil {
		return nil, err
	}

	return link, nil
}

func (s *Session) newLink(ctx context.Context, share *Share, parent *Link, pLink *proton.Link) (*Link, error) {
	slog.Debug("session.newLink", "linkID", pLink.LinkID)
	var err error

	link := Link{
		Type:           pLink.Type,
		Size:           0,
		State:          &pLink.State,
		CreateTime:     pLink.CreateTime,
		ModifyTime:     pLink.ModifyTime,
		ExpirationTime: pLink.ExpirationTime,

		parentLink: parent,
		protonLink: pLink,
		share:      share,
		session:    s,
	}

	link.keyRing, err = link.getKeyRing(pLink.SignatureEmail)
	if err != nil {
		return nil, err
	}

	link.nameKeyRing, err = link.getKeyRing(pLink.NameSignatureEmail)
	if err != nil {
		return nil, err
	}

	link.Name, err = link.getName()
	if err != nil {
		return nil, err
	}

	slog.Debug("session.newLink", "name", link.Name)

	if pLink.Type == proton.LinkTypeFile {
		slog.Debug("session.newLink", "linkType", "file")
		link.Size = pLink.FileProperties.ActiveRevision.Size
		link.ModifyTime = pLink.FileProperties.ActiveRevision.CreateTime
		/*
			link.XAttr, err = pLink.FileProperties.ActiveRevision.GetDecXAttrString(link.parentLink.keyRing, link.keyRing)
			if err != nil {
				return nil, err
			} /**/
	} else {
		slog.Debug("share.session", "linkType", "file")
	}

	return &link, nil
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

	keypass, err := Base64Decode(config.SaltedKeyPass)
	if err != nil {
		return nil, err
	}

	if err := session.Unlock(string(keypass)); err != nil {
		return nil, err
	}

	return session, nil
}

// SessionSave persists session credentials and the salted keypass to the store.
func SessionSave(store SessionStore, session *Session, keypass []byte) error {
	config := &SessionConfig{
		UID:           session.Auth.UID,
		AccessToken:   session.Auth.AccessToken,
		RefreshToken:  session.Auth.RefreshToken,
		SaltedKeyPass: Base64Encode(keypass),
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
