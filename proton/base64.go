package proton

import (
	"encoding/base64"
)

func Base64Decode(str string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(str)
}

func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
