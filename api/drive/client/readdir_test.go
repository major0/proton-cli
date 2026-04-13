package client_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"github.com/major0/proton-cli/api/drive"
	"github.com/major0/proton-cli/api/drive/client"
	"pgregory.net/rapid"
)

// TestNamedDirEntryConsistency_Property verifies that for child entries
// (excluding . and ..), entry.EntryName equals entry.Link.Name().
//
// **Property 4: NamedDirEntry Consistency**
// **Validates: Requirement 3.1**
func TestNamedDirEntryConsistency_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		nChildren := rapid.IntRange(1, 10).Draw(t, "nChildren")

		// Build a directory with N children using walkResolver.
		r := &walkResolver{
			children: make(map[string][]proton.Link),
			names:    make(map[string]string),
		}

		pChildren := make([]proton.Link, nChildren)
		for i := 0; i < nChildren; i++ {
			childID := fmt.Sprintf("child-%d", i)
			childName := rapid.StringMatching(`[a-zA-Z][a-zA-Z0-9]{0,15}`).Draw(t, fmt.Sprintf("name-%d", i))
			pChildren[i] = proton.Link{LinkID: childID, Type: proton.LinkTypeFile}
			r.names[childID] = childName
		}

		dirID := "test-dir"
		r.children[dirID] = pChildren
		r.names[dirID] = "testdir"

		pShare := &proton.Share{
			ShareMetadata: proton.ShareMetadata{ShareID: "test-share"},
		}
		dirPLink := &proton.Link{LinkID: dirID, Type: proton.LinkTypeFolder}
		dirLink := drive.NewTestLink(dirPLink, nil, nil, r, "testdir")
		share := drive.NewShare(pShare, nil, dirLink, r, "")
		dirLink = drive.NewTestLink(dirPLink, nil, share, r, "testdir")
		share.Link = dirLink

		c := &client.Client{}
		ctx := context.Background()

		for entry := range c.ReaddirNamed(ctx, dirLink) {
			if entry.Err != nil {
				t.Fatalf("unexpected error: %v", entry.Err)
			}

			// Skip . and ..
			if entry.EntryName == "." || entry.EntryName == ".." {
				continue
			}

			// For child entries, EntryName must equal Link.Name().
			linkName, err := entry.Link.Name()
			if err != nil {
				t.Fatalf("Link.Name() error: %v", err)
			}
			if entry.EntryName != linkName {
				t.Fatalf("NamedDirEntry.EntryName=%q != Link.Name()=%q for link %s",
					entry.EntryName, linkName, entry.Link.LinkID())
			}
		}
	})
}

// TestReaddirNamed_DotDotDot verifies that the first two entries from
// ReaddirNamed are "." and ".." with correct link pointers.
func TestReaddirNamed_DotDotDot(t *testing.T) {
	r := &walkResolver{
		children: make(map[string][]proton.Link),
		names:    map[string]string{"dir": "mydir"},
	}

	pShare := &proton.Share{
		ShareMetadata: proton.ShareMetadata{ShareID: "test-share"},
	}
	dirPLink := &proton.Link{LinkID: "dir", Type: proton.LinkTypeFolder}
	dirLink := drive.NewTestLink(dirPLink, nil, nil, r, "mydir")
	share := drive.NewShare(pShare, nil, dirLink, r, "")
	dirLink = drive.NewTestLink(dirPLink, nil, share, r, "mydir")
	share.Link = dirLink

	c := &client.Client{}
	ctx := context.Background()

	var entries []client.NamedDirEntry
	for entry := range c.ReaddirNamed(ctx, dirLink) {
		entries = append(entries, entry)
	}

	if len(entries) < 2 {
		t.Fatalf("expected at least 2 entries (. and ..), got %d", len(entries))
	}

	if entries[0].EntryName != "." {
		t.Fatalf("first entry should be '.', got %q", entries[0].EntryName)
	}
	if entries[0].Link != dirLink {
		t.Fatal("first entry (.) should point to the directory itself")
	}

	if entries[1].EntryName != ".." {
		t.Fatalf("second entry should be '..', got %q", entries[1].EntryName)
	}
	// For share root, parent == self.
	if entries[1].Link != dirLink.Parent() {
		t.Fatal("second entry (..) should point to Parent()")
	}
}
