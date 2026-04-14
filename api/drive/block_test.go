package drive

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/ProtonMail/go-proton-api"
	"pgregory.net/rapid"
)

// TestBlockFromProton_Property verifies that BlockFromProton preserves
// Index, BareURL, Token, and correctly decodes the base64 Hash.
//
// **Property 1: Block mapping preserves fields**
// **Validates: Requirements 1.1**
func TestBlockFromProton_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		index := rapid.IntRange(1, 100).Draw(t, "index")
		bareURL := rapid.String().Draw(t, "bareURL")
		token := rapid.String().Draw(t, "token")

		// Generate random hash bytes (32 bytes like SHA-256).
		hashBytes := make([]byte, 32)
		for i := range hashBytes {
			hashBytes[i] = byte(rapid.IntRange(0, 255).Draw(t, fmt.Sprintf("hash-%d", i)))
		}
		hashB64 := base64.StdEncoding.EncodeToString(hashBytes)

		pb := proton.Block{
			Index:   index,
			BareURL: bareURL,
			Token:   token,
			Hash:    hashB64,
		}

		block, err := BlockFromProton(pb)
		if err != nil {
			t.Fatalf("BlockFromProton: %v", err)
		}

		if block.Index != index {
			t.Fatalf("Index = %d, want %d", block.Index, index)
		}
		if block.BareURL != bareURL {
			t.Fatalf("BareURL = %q, want %q", block.BareURL, bareURL)
		}
		if block.Token != token {
			t.Fatalf("Token = %q, want %q", block.Token, token)
		}
		if !bytes.Equal(block.Hash, hashBytes) {
			t.Fatalf("Hash mismatch: got %x, want %x", block.Hash, hashBytes)
		}
	})
}

func TestBlockCount(t *testing.T) {
	tests := []struct {
		size int64
		want int
	}{
		{0, 0},
		{-1, 0},
		{1, 1},
		{BlockSize, 1},
		{BlockSize + 1, 2},
		{BlockSize * 3, 3},
		{BlockSize*3 + 1, 4},
	}
	for _, tt := range tests {
		got := BlockCount(tt.size)
		if got != tt.want {
			t.Errorf("BlockCount(%d) = %d, want %d", tt.size, got, tt.want)
		}
	}
}
