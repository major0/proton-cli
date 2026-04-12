package client_test

import (
	"context"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/major0/proton-cli/api"
	"github.com/major0/proton-cli/api/drive"
	"github.com/major0/proton-cli/api/drive/client"
	"pgregory.net/rapid"
)

// mockResolver is a minimal LinkResolver for testing Remove logic
// without real API calls.
type mockResolver struct{}

func (m *mockResolver) ListLinkChildren(_ context.Context, _, _ string, _ bool) ([]proton.Link, error) {
	return nil, nil
}

func (m *mockResolver) NewChildLink(_ context.Context, parent *drive.Link, pLink *proton.Link) *drive.Link {
	return drive.NewLink(pLink, parent, parent.Share(), m)
}

func (m *mockResolver) AddressForEmail(_ string) (proton.Address, bool) {
	return proton.Address{}, false
}

func (m *mockResolver) AddressKeyRing(_ string) (*crypto.KeyRing, bool) {
	return nil, false
}

func (m *mockResolver) Throttle() *api.Throttle { return nil }
func (m *mockResolver) MaxWorkers() int          { return 1 }

// TestRemove_ShareRoot_Property verifies that Remove rejects share root
// links for any RemoveOpts combination.
// **Property 3: Remove Rejects Share Root**
// **Validates: Requirement 3.2**
func TestRemove_ShareRoot_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		recursive := rapid.Bool().Draw(t, "recursive")
		permanent := rapid.Bool().Draw(t, "permanent")

		resolver := &mockResolver{}
		pShare := &proton.Share{
			ShareMetadata: proton.ShareMetadata{ShareID: "test-share"},
		}
		rootPLink := &proton.Link{LinkID: "root-link", Type: proton.LinkTypeFolder}
		rootLink := drive.NewLink(rootPLink, nil, nil, resolver)
		share := drive.NewShare(pShare, nil, rootLink, resolver)

		// Share root link: ParentLink() == nil.
		linkPLink := &proton.Link{LinkID: "root", Type: proton.LinkTypeFolder}
		link := drive.NewLink(linkPLink, nil, share, resolver)

		// Client with nil Session — Remove returns before accessing it.
		c := &client.Client{}

		err := c.Remove(context.Background(), share, link, drive.RemoveOpts{
			Recursive: recursive,
			Permanent: permanent,
		})

		if err == nil {
			t.Fatal("expected error for share root, got nil")
		}
	})
}
