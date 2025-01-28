package proton

import (
	"context"
	"log/slog"
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

	MaxWorkers int

	addresses      map[string]proton.Address
	AddressKeyRing map[string]*crypto.KeyRing

	user        proton.User
	UserKeyRing *crypto.KeyRing
}

/* Initialize a new session frmo the provided credentials. The session is
 * not fully usable until it has been Unlock()'ed using the user-provided
 * keypass */
func SessionFromCredentials(ctx context.Context, options []proton.Option, creds *SessionCredentials) (*Session, error) {
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

	slog.Debug("refresh client")

	session.manager = proton.New(options...)

	slog.Debug("config", "uid", creds.UID, "access_token", creds.AccessToken, "refresh_token", creds.RefreshToken)
	session.Client = session.manager.NewClient(creds.UID, creds.AccessToken, creds.RefreshToken)

	slog.Debug("GetUser")
	session.user, err = session.Client.GetUser(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("GetAddresses")

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

/* Initialize a new session from the provided login/password. The returned
 * session may have extra authentication requirements, such as 2FA.
 * Once all authentication challenges have been met, the session will still
 * need to be Unlock()'ed to gain access to the User and Address
 * keyrings. */
func SessionFromLogin(ctx context.Context, options []proton.Option, username string, password string) (*Session, error) {
	var err error
	session := &Session{}
	session.MaxWorkers = 10
	session.manager = proton.New(options...)
	slog.Debug("login", "username", username, "password", "<hidden>")
	session.Client, session.Auth, err = session.manager.NewClientWithLogin(ctx, username, []byte(password))
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

	slog.Debug("ListShares", "shares", len(pshares))
	slog.Debug("ListShares", "volumID", volumeID)

	var wg sync.WaitGroup
	idQueue := make(chan string)
	shareQueue := make(chan Share)
	for i := 0; i < min(s.MaxWorkers, len(pshares)); i++ {
		wg.Add(1)
		go func() {
			slog.Debug("starting worker", "id", i)
			defer wg.Done()
			for {
				id, ok := <-idQueue
				if !ok {
					slog.Debug("ending worker", "id", i)
					return
				}

				slog.Debug("worker", "operation", "get", "ShareID", id)
				share, err := s.GetShare(ctx, id)
				if err != nil {
					slog.Error("worker", "shareID", id, "error", err)
				}
				shareQueue <- share
				slog.Debug("worker", "operation", "got", "id", id)
			}
		}()
	}

	/* Spawn a producer to feed the idQueue as fast as the workers can
	 * consume it. */
	wg.Add(1)
	go func() {
		slog.Debug("starting producer")
		defer wg.Done()
		for _, s := range pshares {
			if volumeID != "" && volumeID != s.VolumeID {
				slog.Debug("producer", "operation", "skip", "id", s.ShareID)
				continue
			}
			slog.Debug("producer", "operation", "put", "id", s.ShareID)
			idQueue <- s.ShareID
		}
		/* Let the workers know there is nothing more to be written to the
		 * queue */
		close(idQueue)
		slog.Debug("ending producer")
	}()

	/* Spawn a go routine that waits for all the workers to be
	 * finished and then closes the shareQueue. This acts to signal
	 * the main thread that all the workers are done. Until then the
	 * main thread can `select` the shareQueue appending shares to an
	 * array. */
	go func() {
		slog.Debug("ListShares", "sync", "wait")
		wg.Wait()
		close(shareQueue)
		slog.Debug("ListShares", "sync", "done")
	}()

	var shares []Share
	for {
		share, ok := <-shareQueue
		if !ok {
			for s := range shareQueue {
				slog.Debug("ListShares", "share", s.ProtonShare.ShareID)
				shares = append(shares, s)
			}
			break
		}
		slog.Debug("ListShares", "share", share.ProtonShare.ShareID)
		shares = append(shares, share)
	}

	slog.Debug("ListShares", "workers", "done")

	return shares, nil
}

func (s *Session) GetShare(ctx context.Context, id string) (Share, error) {
	pShare, err := s.Client.GetShare(ctx, id)
	if err != nil {
		return Share{}, err
	}

	shareAddrKR := s.AddressKeyRing[pShare.AddressID]
	shareKR, err := pShare.GetKeyRing(shareAddrKR)
	if err != nil {
		return Share{}, err
	}

	// Do not use s.GetLink() here as it will call s.Client.GetShare()
	// all over again.
	pLink, err := s.Client.GetLink(ctx, pShare.ShareID, pShare.LinkID)
	if err != nil {
		return Share{}, err
	}

	name, err := pLink.GetName(shareKR, shareAddrKR)
	if err != nil {
		return Share{}, err
	}

	var xattr *proton.RevisionXAttrCommon
	if pLink.Type == proton.LinkTypeFile {
		xattr, err = pLink.FileProperties.ActiveRevision.GetDecXAttrString(shareKR, shareAddrKR)
		if err != nil {
			return Share{}, err
		}
	}

	link := Link{
		Name: name,

		Type: pLink.Type,

		XAttr: xattr,

		Size: pLink.Size,

		CreateTime:     pLink.CreateTime,
		ModifyTime:     pLink.ModifyTime,
		ExpirationTime: pLink.ExpirationTime,

		ProtonLink: &pLink,
		session:    s,
	}

	if proton.LinkType(link.Type) == proton.LinkTypeFile {
		link.Size = pLink.FileProperties.ActiveRevision.Size
		link.ModifyTime = pLink.FileProperties.ActiveRevision.CreateTime
	}

	share := Share{
		session:     s,
		Link:        &link,
		ProtonShare: &pShare,
		shareAddrKR: shareAddrKR,
		shareKR:     shareKR,
	}

	return share, nil
}

func (s *Session) GetLink(ctx context.Context, shareID string, linkID string) (Link, error) {
	pLink, err := s.Client.GetLink(ctx, shareID, linkID)
	if err != nil {
		return Link{}, err
	}

	var parentAddrKR *crypto.KeyRing
	if pLink.ParentLinkID == "" {
		pShare, err := s.Client.GetShare(ctx, shareID)
		if err != nil {
			return Link{}, err
		}

		parentAddrKR = s.AddressKeyRing[pShare.AddressID]
	} else {
		parentLink, err := s.Client.GetLink(ctx, shareID, pLink.ParentLinkID)
		if err != nil {
			return Link{}, err
		}

		parentAddrKR = s.AddressKeyRing[parentLink.SignatureEmail]
	}

	linkAddrKR := s.AddressKeyRing[pLink.SignatureEmail]

	parentKR, err := pLink.GetKeyRing(parentAddrKR, linkAddrKR)
	if err != nil {
		return Link{}, err
	}

	name, err := pLink.GetName(parentKR, linkAddrKR)
	if err != nil {
		return Link{}, err
	}

	var xattr *proton.RevisionXAttrCommon
	if pLink.Type == proton.LinkTypeFile {
		xattr, err = pLink.FileProperties.ActiveRevision.GetDecXAttrString(parentKR, linkAddrKR)
		if err != nil {
			return Link{}, err
		}
	}

	link := Link{
		Name: name,

		Type: pLink.Type,

		XAttr: xattr,

		CreateTime:     pLink.CreateTime,
		ModifyTime:     pLink.ModifyTime,
		ExpirationTime: pLink.ExpirationTime,

		ProtonLink: &pLink,
		session:    s,
	}

	if proton.LinkType(link.Type) == proton.LinkTypeFile {
		link.ModifyTime = pLink.FileProperties.ActiveRevision.CreateTime
		link.Size = pLink.FileProperties.ActiveRevision.Size
	}

	return link, nil
}
