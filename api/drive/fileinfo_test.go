package drive

import (
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"pgregory.net/rapid"
)

// TestStatConsistency_Property verifies that Stat() returns a FileInfo
// whose fields match the corresponding Link accessors.
//
// **Property 2: Stat() produces consistent FileInfo**
// **Validates: Requirements 1.2, 1.3**
func TestStatConsistency_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		linkID := rapid.String().Draw(t, "linkID")
		mimeType := rapid.String().Draw(t, "mimeType")
		modTime := rapid.Int64().Draw(t, "modTime")
		createTime := rapid.Int64().Draw(t, "createTime")
		isDir := rapid.Bool().Draw(t, "isDir")
		testName := rapid.StringMatching(`[a-zA-Z0-9]{1,20}`).Draw(t, "name")

		lt := proton.LinkTypeFile
		if isDir {
			lt = proton.LinkTypeFolder
		}

		pLink := &proton.Link{
			LinkID:     linkID,
			MIMEType:   mimeType,
			ModifyTime: modTime,
			CreateTime: createTime,
			Type:       lt,
		}

		resolver := &mockLinkResolver{}
		link := NewTestLink(pLink, nil, nil, resolver, testName)

		fi := link.Stat()

		if fi.LinkID != linkID {
			t.Fatalf("LinkID = %q, want %q", fi.LinkID, linkID)
		}
		if fi.MIMEType != mimeType {
			t.Fatalf("MIMEType = %q, want %q", fi.MIMEType, mimeType)
		}
		if fi.CreateTime != createTime {
			t.Fatalf("CreateTime = %d, want %d", fi.CreateTime, createTime)
		}
		if fi.IsDir != isDir {
			t.Fatalf("IsDir = %v, want %v", fi.IsDir, isDir)
		}

		// Name() should match testName for test links.
		name, err := fi.Name()
		if err != nil {
			t.Fatalf("Name(): %v", err)
		}
		if name != testName {
			t.Fatalf("Name() = %q, want %q", name, testName)
		}
	})
}
