package proton

import (
	"context"

	"github.com/ProtonMail/go-proton-api"
)

type Volume struct {
	session *Session
	pVolume proton.Volume
	//shares  []Share
}

/* ListShareMetadata returns the metadata for all shares in the volume.
 * This is distinct from Client.ListShares() which returns a list of
 * all shares for all volumes. This could be the result of a share from a
 * different user volume. */
func (v *Volume) ListShareMetadata(ctx context.Context, all bool) ([]proton.ShareMetadata, error) {
	pshares, err := v.session.Client.ListShares(ctx, all)
	if err != nil {
		return nil, err
	}

	var shares []proton.ShareMetadata
	for _, s := range pshares {
		if s.VolumeID == v.pVolume.VolumeID {
			shares = append(shares, s)
		}
	}

	return shares, nil
}

/* GetShareMetadata returns the metadata for the given share. There does
 * not appear to be a way to this directly from the API.
 * Calling "/drive/shares/<id>" returns an object of type PShare, where
 * calling "/drive/shares" returns a list of objcts of type
 * PShareMetadata. That means that we effectively need to find the target
 * share manually by sifting the list. */
func (v *Volume) GetShareMetadata(ctx context.Context, id string, all bool) (proton.ShareMetadata, error) {
	pshares, err := v.session.Client.ListShares(ctx, all)
	if err != nil {
		return proton.ShareMetadata{}, err
	}

	for _, s := range pshares {
		if s.ShareID == id && s.VolumeID == v.pVolume.VolumeID {
			return s, nil
		}
	}

	return proton.ShareMetadata{}, nil
}

/* GetShare returns the share with the given id. */
func (v *Volume) GetShare(ctx context.Context, id string) (*Share, error) {
	share, err := v.session.GetShare(ctx, id)
	if err != nil {
		return nil, err
	}

	if share.protonShare.VolumeID != v.pVolume.VolumeID {
		return nil, nil
	}

	return share, nil
}
