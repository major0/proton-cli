package client

import (
	"context"
	"fmt"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api/drive"
)

// FileHandle holds the resolved state needed to populate a CopyEndpoint
// for a Proton Drive file. Returned by CreateFile (for destinations)
// and OpenFile (for sources).
type FileHandle struct {
	Link       *drive.Link
	Share      *drive.Share
	RevisionID string
	Blocks     []proton.Block     // populated by OpenFile (source)
	SessionKey *crypto.SessionKey // for encrypt (dest) or decrypt (source)
	FileSize   int64              // populated by OpenFile (source)
	BlockSizes []int64            // populated by OpenFile (source)
}

// CreateFile creates a file draft in Proton Drive and returns a
// FileHandle with the RevisionID and SessionKey needed for upload.
// The caller uses these to populate a CopyEndpoint destination.
func (c *Client) CreateFile(ctx context.Context, share *drive.Share, parentLink *drive.Link, name string) (*FileHandle, error) {
	mimeType := detectMIMEType(name)

	parentKR, err := parentLink.KeyRing()
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: parent keyring: %w", err)
	}

	addrKR, err := c.addrKRForLink(parentLink)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: address keyring: %w", err)
	}

	sigAddr, err := c.signatureAddress(parentLink)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: signature address: %w", err)
	}

	nodeKey, encPassphrase, passphraseSig, err := generateNodeKeys(parentKR, addrKR)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: generate node keys: %w", err)
	}

	req := proton.CreateFileReq{
		ParentLinkID:            parentLink.ProtonLink().LinkID,
		MIMEType:                mimeType,
		NodeKey:                 nodeKey,
		NodePassphrase:          encPassphrase,
		NodePassphraseSignature: passphraseSig,
		SignatureAddress:        sigAddr,
	}

	nodeKR, err := unlockKeyRing(parentKR, addrKR, nodeKey, encPassphrase, passphraseSig)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: unlock node keyring: %w", err)
	}

	sessionKey, err := req.SetContentKeyPacketAndSignature(nodeKR)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: content key packet: %w", err)
	}

	if err := req.SetName(name, addrKR, nodeKR); err != nil {
		return nil, fmt.Errorf("drive.CreateFile: encrypt name: %w", err)
	}

	hashKey, err := parentLink.ProtonLink().GetHashKeyFromParent(parentKR, addrKR)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: hash key: %w", err)
	}
	if err := req.SetHash(name, hashKey); err != nil {
		return nil, fmt.Errorf("drive.CreateFile: name hash: %w", err)
	}

	shareID := share.ProtonShare().ShareID
	res, err := c.Session.Client.CreateFile(ctx, shareID, req)
	if err != nil {
		return nil, fmt.Errorf("drive.CreateFile: %w", err)
	}

	return &FileHandle{
		Link:       parentLink,
		Share:      share,
		RevisionID: res.RevisionID,
		SessionKey: sessionKey,
	}, nil
}

// OpenFile prepares a Proton Drive file for reading by fetching the
// revision block list and deriving the session key. Returns a
// FileHandle with the info needed to populate a CopyEndpoint source.
func (c *Client) OpenFile(ctx context.Context, link *drive.Link) (*FileHandle, error) {
	if link.Type() != proton.LinkTypeFile {
		return nil, fmt.Errorf("drive.OpenFile: %s: not a file", link.LinkID())
	}

	pLink := link.ProtonLink()
	if pLink.FileProperties == nil {
		return nil, fmt.Errorf("drive.OpenFile: %s: no file properties", link.LinkID())
	}

	shareID := link.Share().ProtonShare().ShareID
	revisionID := pLink.FileProperties.ActiveRevision.ID

	revision, err := c.Session.Client.GetRevisionAllBlocks(ctx, shareID, link.LinkID(), revisionID)
	if err != nil {
		return nil, fmt.Errorf("drive.OpenFile: %s: get revision: %w", link.LinkID(), err)
	}

	nodeKR, err := link.KeyRing()
	if err != nil {
		return nil, fmt.Errorf("drive.OpenFile: %s: keyring: %w", link.LinkID(), err)
	}

	sessionKey, err := pLink.GetSessionKey(nodeKR)
	if err != nil {
		return nil, fmt.Errorf("drive.OpenFile: %s: session key: %w", link.LinkID(), err)
	}

	// Compute block sizes from revision XAttr if available.
	var blockSizes []int64
	addrKR, err := c.addrKRForLink(link)
	if err == nil {
		xattr, xErr := revision.GetDecXAttrString(addrKR, nodeKR)
		if xErr == nil && xattr != nil {
			blockSizes = xattr.BlockSizes
		}
	}

	fileSize := pLink.FileProperties.ActiveRevision.Size

	// Fall back to computing block sizes from file size.
	if len(blockSizes) == 0 && fileSize > 0 {
		n := drive.BlockCount(fileSize)
		blockSizes = make([]int64, n)
		remaining := fileSize
		for i := range blockSizes {
			if remaining >= drive.BlockSize {
				blockSizes[i] = drive.BlockSize
			} else {
				blockSizes[i] = remaining
			}
			remaining -= blockSizes[i]
		}
	}

	return &FileHandle{
		Link:       link,
		Share:      link.Share(),
		RevisionID: revisionID,
		Blocks:     revision.Blocks,
		SessionKey: sessionKey,
		FileSize:   fileSize,
		BlockSizes: blockSizes,
	}, nil
}
