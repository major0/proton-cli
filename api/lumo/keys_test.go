package lumo

import (
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func TestLumoPubKeyValid(t *testing.T) {
	key, err := crypto.NewKeyFromArmored(LumoPubKeyProd)
	if err != nil {
		t.Fatalf("parse LumoPubKeyProd: %v", err)
	}
	fp := key.GetFingerprint()
	const want = "f032a1169ddff8eda728e59a9a74c3ef61514a2a"
	if fp != want {
		t.Fatalf("fingerprint = %q, want %q", fp, want)
	}
}
