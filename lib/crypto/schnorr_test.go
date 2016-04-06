package crypto

import (
	"testing"

	"github.com/dedis/crypto/config"
	"github.com/dedis/crypto/edwards/ed25519"
)

func TestSchnorrSignature(t *testing.T) {
	msg := []byte("Hello Schnorr")
	suite := ed25519.NewAES128SHA256Ed25519(false)
	kp := config.NewKeyPair(suite)

	s, err := SignSchnorr(suite, kp.Secret, msg)
	if err != nil {
		t.Fatalf("Couldn't sign msg: %s: %v", msg, err)
	}
	err = VerifySchnorr(suite, kp.Public, msg, s)
	if err != nil {
		t.Fatalf("Couldn't verify signature: \n%+v\nfor msg:'%s'. Error:\n%v", s, msg, err)
	}
}
