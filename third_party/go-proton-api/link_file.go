package proton

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-resty/resty/v2"
)

// GetRevisionVerification fetches the verification code for a draft revision from the
// v2 volume-based API. The VerificationCode is XOR'd with each block's encrypted bytes
// to produce a per-block Verifier.Token that the storage backend requires.
func (c *Client) GetRevisionVerification(ctx context.Context, volumeID, linkID, revisionID string) (RevisionVerification, error) {
	var res struct {
		VerificationCode string
		ContentKeyPacket string
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get(
			"/drive/v2/volumes/" + volumeID + "/links/" + linkID + "/revisions/" + revisionID + "/verification",
		)
	}); err != nil {
		return RevisionVerification{}, err
	}

	return RevisionVerification{
		VerificationCode: res.VerificationCode,
		ContentKeyPacket: res.ContentKeyPacket,
	}, nil
}

func (c *Client) ListRevisions(ctx context.Context, shareID, linkID string) ([]RevisionMetadata, error) {
	var res struct {
		Revisions []RevisionMetadata
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/shares/" + shareID + "/files/" + linkID + "/revisions")
	}); err != nil {
		return nil, err
	}

	return res.Revisions, nil
}

func (c *Client) GetRevisionAllBlocks(ctx context.Context, shareID, linkID, revisionID string) (Revision, error) {
	var res struct {
		Revision Revision
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.
			SetResult(&res).
			Get("/drive/shares/" + shareID + "/files/" + linkID + "/revisions/" + revisionID)
	}); err != nil {
		return Revision{}, err
	}

	return res.Revision, nil
}

func (c *Client) GetRevision(ctx context.Context, shareID, linkID, revisionID string, fromBlock, pageSize int) (Revision, error) {
	if fromBlock < 1 {
		return Revision{}, fmt.Errorf("fromBlock must be greater than 0")
	} else if pageSize < 1 {
		return Revision{}, fmt.Errorf("pageSize must be greater than 0")
	}

	var res struct {
		Revision Revision
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.
			SetQueryParams(map[string]string{
				"FromBlockIndex": strconv.Itoa(fromBlock),
				"PageSize":       strconv.Itoa(pageSize),
			}).
			SetResult(&res).
			Get("/drive/shares/" + shareID + "/files/" + linkID + "/revisions/" + revisionID)
	}); err != nil {
		return Revision{}, err
	}

	return res.Revision, nil
}

func (c *Client) UpdateRevision(ctx context.Context, shareID, linkID, revisionID string, req UpdateRevisionReq) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(req).Put("/drive/shares/" + shareID + "/files/" + linkID + "/revisions/" + revisionID)
	})
}

func (c *Client) DeleteRevision(ctx context.Context, shareID, linkID, revisionID string) error {
	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.Delete("/drive/shares/" + shareID + "/files/" + linkID + "/revisions/" + revisionID)
	})
}

func (c *Client) CreateRevision(ctx context.Context, shareID, linkID string) (CreateRevisionRes, error) {
	var res struct {
		Revision CreateRevisionRes
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Post("/drive/shares/" + shareID + "/files/" + linkID + "/revisions")
	}); err != nil {
		return CreateRevisionRes{}, err
	}

	return res.Revision, nil
}
