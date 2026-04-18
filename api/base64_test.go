package api

import (
	"bytes"
	"testing"

	"pgregory.net/rapid"
)

// TestBase64EncodeDecode verifies basic encode/decode behavior.
func TestBase64EncodeDecode(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"single byte", []byte{0x42}},
		{"hello", []byte("hello world")},
		{"binary", []byte{0x00, 0xff, 0x80, 0x7f}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := Base64Encode(tt.input)
			decoded, err := Base64Decode(encoded)
			if err != nil {
				t.Fatalf("Base64Decode: %v", err)
			}
			if !bytes.Equal(decoded, tt.input) {
				t.Fatalf("round-trip failed: got %v, want %v", decoded, tt.input)
			}
		})
	}
}

// TestBase64Decode_Invalid verifies that invalid base64 returns an error.
func TestBase64Decode_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"invalid chars", "!!!not-base64!!!", true},
		{"truncated", "YQ", true}, // missing padding
		{"valid", "aGVsbG8=", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Base64Decode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Base64Decode(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// TestBase64RoundTrip_Property verifies that for any byte slice,
// Base64Encode followed by Base64Decode returns the original data.
//
// **Validates: Requirements 2.3**
func TestBase64RoundTrip_Property(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		data := rapid.SliceOf(rapid.Byte()).Draw(t, "data")

		encoded := Base64Encode(data)
		decoded, err := Base64Decode(encoded)
		if err != nil {
			t.Fatalf("Base64Decode: %v", err)
		}

		if !bytes.Equal(decoded, data) {
			t.Fatalf("round-trip failed: len(input)=%d, len(output)=%d", len(data), len(decoded))
		}
	})
}
