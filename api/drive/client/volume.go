package client

import (
	"context"

	"github.com/major0/proton-cli/api/drive"
)

// ListVolumes returns all volumes accessible by this session.
func (c *Client) ListVolumes(ctx context.Context) ([]drive.Volume, error) {
	pVolumes, err := c.Session.Client.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	volumes := make([]drive.Volume, len(pVolumes))
	for i := range pVolumes {
		volumes[i] = drive.Volume{ProtonVolume: pVolumes[i]}
	}

	return volumes, nil
}

// GetVolume returns the volume with the given ID.
func (c *Client) GetVolume(ctx context.Context, id string) (drive.Volume, error) {
	pVolume, err := c.Session.Client.GetVolume(ctx, id)
	if err != nil {
		return drive.Volume{}, err
	}

	return drive.Volume{ProtonVolume: pVolume}, nil
}
