package client

import (
	"context"
	"fmt"
	"mime"
	"os"
	"path/filepath"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
)

// UploadFile uploads a local file to Proton Drive using the block pipeline.
// Creates a file draft, feeds blocks through the transferPipeline, then
// commits the revision.
func (c *Client) UploadFile(ctx context.Context, share *drive.Share, parentLink *drive.Link, localPath string, opts TransferOpts) error {
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: stat %s: %w", localPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("drive.UploadFile: %s: is a directory", localPath)
	}

	fileName := filepath.Base(localPath)
	mimeType := detectMIMEType(fileName)

	// Get parent keyring for encrypting the file name and node keys.
	parentKR, err := parentLink.KeyRing()
	if err != nil {
		return fmt.Errorf("drive.UploadFile: parent keyring: %w", err)
	}

	addrKR, err := c.addrKRForLink(parentLink)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: address keyring: %w", err)
	}

	sigAddr, err := c.signatureAddress(parentLink)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: signature address: %w", err)
	}

	// Generate node keys for the new file.
	nodeKey, encPassphrase, passphraseSig, err := generateNodeKeys(parentKR, addrKR)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: generate node keys: %w", err)
	}

	// Build the CreateFile request.
	req := proton.CreateFileReq{
		ParentLinkID:           parentLink.ProtonLink().LinkID,
		MIMEType:               mimeType,
		NodeKey:                nodeKey,
		NodePassphrase:         encPassphrase,
		NodePassphraseSignature: passphraseSig,
		SignatureAddress:       sigAddr,
	}

	// Unlock the node keyring to set content key packet and encrypt name.
	nodeKR, err := unlockKeyRing(parentKR, addrKR, nodeKey, encPassphrase, passphraseSig)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: unlock node keyring: %w", err)
	}

	sessionKey, err := req.SetContentKeyPacketAndSignature(nodeKR)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: content key packet: %w", err)
	}

	if err := req.SetName(fileName, addrKR, nodeKR); err != nil {
		return fmt.Errorf("drive.UploadFile: encrypt name: %w", err)
	}

	hashKey, err := parentLink.ProtonLink().GetHashKeyFromParent(parentKR, addrKR)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: hash key: %w", err)
	}
	if err := req.SetHash(fileName, hashKey); err != nil {
		return fmt.Errorf("drive.UploadFile: name hash: %w", err)
	}

	// Create the file draft.
	shareID := share.ProtonShare().ShareID
	res, err := c.Session.Client.CreateFile(ctx, shareID, req)
	if err != nil {
		return fmt.Errorf("drive.UploadFile: create file: %w", err)
	}

	// Build the CopyJob.
	fileSize := info.Size()
	job := CopyJob{
		Src: CopyEndpoint{
			Type:      PathLocal,
			LocalPath: localPath,
			FileSize:  fileSize,
		},
		Dst: CopyEndpoint{
			Type:       PathProton,
			Link:       parentLink,
			Share:      share,
			RevisionID: res.RevisionID,
			SessionKey: sessionKey,
			FileSize:   fileSize,
		},
	}

	store := NewBlockStore(c.Session, nil) // TODO: wire cache from share config
	pipe := &transferPipeline{
		workers: opts.workers(),
		store:   store,
		client:  c,
	}

	if err := pipe.run(ctx, []CopyJob{job}); err != nil {
		return fmt.Errorf("drive.UploadFile: %s: %w", fileName, err)
	}

	// TODO: after pipeline completes:
	// 1. Compute SHA-256 per encrypted block, SHA-1 of file content
	// 2. Sign block hash manifest with address keyring
	// 3. Commit revision via UpdateRevision with encrypted XAttr
	// These require collecting block hashes from the writer workers,
	// which needs pipeline enhancement to return per-block metadata.

	return nil
}

// detectMIMEType returns the MIME type for a file name based on extension.
// Returns "application/octet-stream" for unknown extensions.
func detectMIMEType(name string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		return "application/octet-stream"
	}
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		return "application/octet-stream"
	}
	return mimeType
}
