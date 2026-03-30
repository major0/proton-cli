package proton

import (
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"

	protonapi "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
)

const uploadBlockSize = 4 * 1024 * 1024

type UploadResult struct {
	ShareID    string
	LinkID     string
	RevisionID string
	FileName   string
	Size       int64
}

func (s *Session) getAddressByID(id string) (protonapi.Address, error) {
	addr, ok := s.addressesByID[id]
	if !ok {
		return protonapi.Address{}, ErrKeyNotFound
	}
	return addr, nil
}

func (s *Session) getPrimaryShare(ctx context.Context) (*Share, error) {
	meta, err := s.ListSharesMetadata(ctx, true)
	if err != nil {
		return nil, err
	}

	for _, share := range meta {
		ps := protonapi.ShareMetadata(share)
		if ps.Flags == protonapi.PrimaryShare {
			return s.GetShare(ctx, ps.ShareID)
		}
	}

	for _, share := range meta {
		ps := protonapi.ShareMetadata(share)
		if ps.Type == protonapi.ShareTypeMain {
			return s.GetShare(ctx, ps.ShareID)
		}
	}

	return nil, fmt.Errorf("no primary/main share found")
}

func encryptWithDetachedSignature(encKR, sigKR *crypto.KeyRing, data []byte) (string, string, error) {
	plain := crypto.NewPlainMessage(data)

	enc, err := encKR.Encrypt(plain, nil)
	if err != nil {
		return "", "", err
	}
	encArm, err := enc.GetArmored()
	if err != nil {
		return "", "", err
	}

	sig, err := sigKR.SignDetached(plain)
	if err != nil {
		return "", "", err
	}
	sigArm, err := sig.GetArmored()
	if err != nil {
		return "", "", err
	}

	return encArm, sigArm, nil
}

func unlockNodeKeyRing(parentNodeKR, addrKR *crypto.KeyRing, armoredNodeKey, armoredNodePassphrase, armoredSig string) (*crypto.KeyRing, error) {
	enc, err := crypto.NewPGPMessageFromArmored(armoredNodePassphrase)
	if err != nil {
		return nil, err
	}
	dec, err := parentNodeKR.Decrypt(enc, nil, crypto.GetUnixTime())
	if err != nil {
		return nil, err
	}

	sig, err := crypto.NewPGPSignatureFromArmored(armoredSig)
	if err != nil {
		return nil, err
	}
	if err := addrKR.VerifyDetached(dec, sig, crypto.GetUnixTime()); err != nil {
		return nil, err
	}

	lockedKey, err := crypto.NewKeyFromArmored(armoredNodeKey)
	if err != nil {
		return nil, err
	}
	unlockedKey, err := lockedKey.Unlock(dec.GetBinary())
	if err != nil {
		return nil, err
	}
	return crypto.NewKeyRing(unlockedKey)
}

func generateNodeKeys(parentNodeKR, addrKR *crypto.KeyRing, email string) (string, string, string, error) {
	passphrase, err := crypto.RandomToken(32)
	if err != nil {
		return "", "", "", err
	}

	armoredKey, err := helper.GenerateKey("proton-cli", email, passphrase, "rsa", 2048)
	if err != nil {
		return "", "", "", err
	}

	encPassphrase, sigPassphrase, err := encryptWithDetachedSignature(parentNodeKR, addrKR, passphrase)
	if err != nil {
		return "", "", "", err
	}

	return armoredKey, encPassphrase, sigPassphrase, nil
}

func (s *Session) UploadFileToPrimaryShareRoot(ctx context.Context, localPath, remoteName string) (*UploadResult, error) {
	share, err := s.getPrimaryShare(ctx)
	if err != nil {
		return nil, err
	}
	return s.UploadFileToShareRoot(ctx, share.protonShare.ShareID, localPath, remoteName)
}

func (s *Session) UploadFileToShareRoot(ctx context.Context, shareID, localPath, remoteName string) (*UploadResult, error) {
	if remoteName == "" {
		remoteName = filepath.Base(localPath)
	}

	share, err := s.GetShare(ctx, shareID)
	if err != nil {
		return nil, err
	}

	root := share.Link
	if root == nil {
		return nil, fmt.Errorf("share root link is missing")
	}
	if protonapi.LinkType(root.Type) != protonapi.LinkTypeFolder {
		return nil, fmt.Errorf("share root is not a folder")
	}

	info, err := os.Stat(localPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	addr, err := s.getAddressByID(share.protonShare.AddressID)
	if err != nil {
		return nil, err
	}
	addrKR, ok := s.AddressKeyRing[addr.ID]
	if !ok {
		return nil, ErrKeyNotFound
	}

	parentNodeKR := root.keyRing
	if parentNodeKR == nil {
		return nil, fmt.Errorf("missing parent node keyring")
	}

	verifyKR, err := root.getAddrKeyRing(root.protonLink.SignatureEmail)
	if err != nil {
		return nil, err
	}
	parentHashKey, err := root.protonLink.GetHashKey(parentNodeKR, verifyKR)
	if err != nil {
		return nil, err
	}

	armoredNodeKey, armoredNodePassphrase, armoredNodePassphraseSig, err := generateNodeKeys(parentNodeKR, addrKR, addr.Email)
	if err != nil {
		return nil, err
	}

	createReq := protonapi.CreateFileReq{
		ParentLinkID:            root.protonLink.LinkID,
		MIMEType:                mime.TypeByExtension(filepath.Ext(remoteName)),
		NodeKey:                 armoredNodeKey,
		NodePassphrase:          armoredNodePassphrase,
		NodePassphraseSignature: armoredNodePassphraseSig,
		SignatureAddress:        addr.Email,
	}
	if createReq.MIMEType == "" {
		createReq.MIMEType = "application/octet-stream"
	}
	if err := createReq.SetName(remoteName, addrKR, parentNodeKR); err != nil {
		return nil, err
	}
	if err := createReq.SetHash(remoteName, parentHashKey); err != nil {
		return nil, err
	}

	fileNodeKR, err := unlockNodeKeyRing(parentNodeKR, addrKR, armoredNodeKey, armoredNodePassphrase, armoredNodePassphraseSig)
	if err != nil {
		return nil, err
	}
	fileSessionKey, err := createReq.SetContentKeyPacketAndSignature(fileNodeKR)
	if err != nil {
		return nil, err
	}

	createRes, err := s.Client.CreateFile(ctx, shareID, createReq)
	if err != nil {
		if err == protonapi.ErrFileNameExist {
			children, childErr := root.ListChildren(ctx, true)
			if childErr != nil {
				return nil, childErr
			}
			for _, child := range children {
				if child.Name != remoteName {
					continue
				}
				if child.State != nil && *child.State == protonapi.LinkStateDraft {
					if delErr := s.Client.DeleteChildren(ctx, shareID, root.protonLink.LinkID, child.protonLink.LinkID); delErr != nil {
						return nil, delErr
					}
					createRes, err = s.Client.CreateFile(ctx, shareID, createReq)
					if err != nil {
						return nil, err
					}
					goto createdFile
				}
				return nil, fmt.Errorf("file already exists in Proton Drive root: %s", remoteName)
			}
		}
		return nil, err
	}

createdFile:

	revisionVerification, err := s.Client.GetRevisionVerification(ctx, share.protonShare.VolumeID, createRes.ID, createRes.RevisionID)
	if err != nil {
		return nil, fmt.Errorf("get revision verification: %w", err)
	}
	verificationCode, err := base64.StdEncoding.DecodeString(revisionVerification.VerificationCode)
	if err != nil {
		return nil, fmt.Errorf("decode verification code: %w", err)
	}

	manifestSignatureData := make([]byte, 0)
	blockSizes := make([]int64, 0)
	blockTokens := make([]protonapi.BlockToken, 0)
	totalPlainSize := int64(0)
	sha1Digest := sha1.New()

	buffer := make([]byte, uploadBlockSize)
	blockIndex := 1
	for {
		n, readErr := io.ReadFull(f, buffer)
		if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
			return nil, readErr
		}
		if n == 0 {
			break
		}

		plainBlock := make([]byte, n)
		copy(plainBlock, buffer[:n])
		totalPlainSize += int64(n)
		blockSizes = append(blockSizes, int64(n))
		_, _ = sha1Digest.Write(plainBlock)

		plainMsg := crypto.NewPlainMessage(plainBlock)
		encBlock, err := fileSessionKey.Encrypt(plainMsg)
		if err != nil {
			return nil, err
		}
		encSig, err := addrKR.SignDetachedEncrypted(plainMsg, fileNodeKR)
		if err != nil {
			return nil, err
		}
		encSigArm, err := encSig.GetArmored()
		if err != nil {
			return nil, err
		}

		encHashBytes := sha256.Sum256(encBlock)
		manifestSignatureData = append(manifestSignatureData, encHashBytes[:]...)

		verificationToken := make([]byte, len(verificationCode))
		for i, v := range verificationCode {
			var b byte
			if i < len(encBlock) {
				b = encBlock[i]
			}
			verificationToken[i] = v ^ b
		}

		blockInfo := protonapi.BlockUploadInfo{
			Index:        blockIndex,
			Size:         int64(len(encBlock)),
			EncSignature: encSigArm,
			Hash:         base64.StdEncoding.EncodeToString(encHashBytes[:]),
			Verifier: protonapi.BlockUploadVerifier{
				Token: base64.StdEncoding.EncodeToString(verificationToken),
			},
		}

		uploadLinks, err := s.Client.RequestBlockUpload(ctx, protonapi.BlockUploadReq{
			AddressID:  share.protonShare.AddressID,
			ShareID:    shareID,
			LinkID:     createRes.ID,
			RevisionID: createRes.RevisionID,
			BlockList:  []protonapi.BlockUploadInfo{blockInfo},
		})
		if err != nil {
			return nil, err
		}
		if len(uploadLinks) != 1 {
			return nil, fmt.Errorf("expected 1 upload link, got %d", len(uploadLinks))
		}
		if err := s.Client.UploadBlock(ctx, uploadLinks[0].BareURL, uploadLinks[0].Token, encBlock); err != nil {
			return nil, err
		}

		blockTokens = append(blockTokens, protonapi.BlockToken{Index: blockIndex, Token: uploadLinks[0].Token})
		blockIndex++

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			break
		}
	}

	manifestSig, err := addrKR.SignDetached(crypto.NewPlainMessage(manifestSignatureData))
	if err != nil {
		return nil, err
	}
	manifestSigArm, err := manifestSig.GetArmored()
	if err != nil {
		return nil, err
	}

	xattr := &protonapi.RevisionXAttrCommon{
		ModificationTime: info.ModTime().Format("2006-01-02T15:04:05-0700"),
		Size:             totalPlainSize,
		BlockSizes:       blockSizes,
		Digests: map[string]string{
			"SHA1": hex.EncodeToString(sha1Digest.Sum(nil)),
		},
	}

	updateReq := protonapi.UpdateRevisionReq{
		BlockList:         blockTokens,
		State:             protonapi.RevisionStateActive,
		ManifestSignature: manifestSigArm,
		SignatureAddress:  addr.Email,
	}
	if err := updateReq.SetEncXAttrString(addrKR, fileNodeKR, xattr); err != nil {
		return nil, err
	}
	if err := s.Client.UpdateRevision(ctx, shareID, createRes.ID, createRes.RevisionID, updateReq); err != nil {
		return nil, err
	}

	return &UploadResult{
		ShareID:    shareID,
		LinkID:     createRes.ID,
		RevisionID: createRes.RevisionID,
		FileName:   remoteName,
		Size:       totalPlainSize,
	}, nil
}
