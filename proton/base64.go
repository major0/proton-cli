package proton

import (
	"encoding/base64"
)

// Base64Decode decodes a standard base64-encoded string.
func Base64Decode(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

// Base64Encode encodes data as a standard base64 string.
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
