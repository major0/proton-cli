package lumo

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestGenerateRequestKey(t *testing.T) {
	key, err := GenerateRequestKey()
	if err != nil {
		t.Fatalf("GenerateRequestKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("key length = %d, want 32", len(key))
	}
	// Two keys must differ (probabilistic but effectively certain).
	key2, err := GenerateRequestKey()
	if err != nil {
		t.Fatalf("GenerateRequestKey (2nd): %v", err)
	}
	if string(key) == string(key2) {
		t.Fatal("two generated keys are identical")
	}
}

func TestGenerateRequestID(t *testing.T) {
	id := GenerateRequestID()
	if len(id) != 36 {
		t.Fatalf("request ID length = %d, want 36 (UUID format)", len(id))
	}
	// Basic UUID format check: 8-4-4-4-12
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Fatalf("request ID %q does not match UUID format", id)
	}
	// Two IDs must differ.
	id2 := GenerateRequestID()
	if id == id2 {
		t.Fatal("two generated request IDs are identical")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key, err := GenerateRequestKey()
	if err != nil {
		t.Fatal(err)
	}
	wrongKey, err := GenerateRequestKey()
	if err != nil {
		t.Fatal(err)
	}
	ad := []byte("test-ad")
	encoded, err := encryptAESGCM([]byte("hello"), key, ad)
	if err != nil {
		t.Fatal(err)
	}
	_, err = decryptAESGCM(encoded, wrongKey, ad)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestDecryptWithWrongAD(t *testing.T) {
	key, err := GenerateRequestKey()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encryptAESGCM([]byte("hello"), key, []byte("correct-ad"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = decryptAESGCM(encoded, key, []byte("wrong-ad"))
	if err == nil {
		t.Fatal("expected error decrypting with wrong AD")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestZeroKey(t *testing.T) {
	key := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32}
	ZeroKey(key)
	for i, b := range key {
		if b != 0 {
			t.Fatalf("key[%d] = %d, want 0", i, b)
		}
	}
}

func TestEncryptTurns(t *testing.T) {
	key, err := GenerateRequestKey()
	if err != nil {
		t.Fatal(err)
	}
	requestID := GenerateRequestID()

	imgData := base64.StdEncoding.EncodeToString([]byte("fake-image-bytes"))
	turns := []Turn{
		{Role: RoleUser, Content: "hello world"},
		{Role: RoleUser, Content: "with image", Images: []WireImage{
			{ImageID: "img1", Data: imgData},
		}},
	}

	encrypted, err := EncryptTurns(turns, key, requestID)
	if err != nil {
		t.Fatalf("EncryptTurns: %v", err)
	}

	for i, turn := range encrypted {
		if !turn.Encrypted {
			t.Errorf("turn[%d].Encrypted = false, want true", i)
		}
		// Content should be base64 (not the original plaintext).
		if turn.Content == "hello world" || turn.Content == "with image" {
			t.Errorf("turn[%d].Content not encrypted", i)
		}
	}

	// Verify images are encrypted.
	if !encrypted[1].Images[0].Encrypted {
		t.Error("image not marked encrypted")
	}
	if encrypted[1].Images[0].Data == imgData {
		t.Error("image data not encrypted")
	}
}

func TestDecryptTokenData(t *testing.T) {
	key, err := GenerateRequestKey()
	if err != nil {
		t.Fatal(err)
	}
	requestID := GenerateRequestID()
	ad := []byte(ResponseAD(requestID))

	original := "decrypted token content"
	encoded, err := encryptAESGCM([]byte(original), key, ad)
	if err != nil {
		t.Fatal(err)
	}

	msg := &GenerationResponseMessage{
		Type:      "token_data",
		Content:   encoded,
		Encrypted: true,
	}
	if err := DecryptTokenData(msg, key, requestID); err != nil {
		t.Fatalf("DecryptTokenData: %v", err)
	}
	if msg.Content != original {
		t.Fatalf("Content = %q, want %q", msg.Content, original)
	}
	if msg.Encrypted {
		t.Fatal("Encrypted should be false after decryption")
	}
}

func TestDecryptImageData(t *testing.T) {
	key, err := GenerateRequestKey()
	if err != nil {
		t.Fatal(err)
	}
	requestID := GenerateRequestID()
	ad := []byte(ResponseAD(requestID))

	originalBytes := []byte("raw-image-data")
	encoded, err := encryptAESGCM(originalBytes, key, ad)
	if err != nil {
		t.Fatal(err)
	}

	msg := &GenerationResponseMessage{
		Type:      "image_data",
		Data:      encoded,
		Encrypted: true,
	}
	if err := DecryptImageData(msg, key, requestID); err != nil {
		t.Fatalf("DecryptImageData: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if string(decoded) != string(originalBytes) {
		t.Fatalf("Data = %q, want %q", decoded, originalBytes)
	}
	if msg.Encrypted {
		t.Fatal("Encrypted should be false after decryption")
	}
}

func TestDecryptTokenData_NotEncrypted(t *testing.T) {
	msg := &GenerationResponseMessage{
		Type:      "token_data",
		Content:   "plain text",
		Encrypted: false,
	}
	if err := DecryptTokenData(msg, nil, "any-id"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Content != "plain text" {
		t.Fatalf("Content changed for non-encrypted message")
	}
}
