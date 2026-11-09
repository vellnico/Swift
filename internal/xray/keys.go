package xray

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

// RealityKeyPair returns an x25519 key pair in the raw base64url form Xray uses.
func RealityKeyPair() (private, public string, err error) {
	priv := make([]byte, curve25519.ScalarSize)
	if _, err := rand.Read(priv); err != nil {
		return "", "", fmt.Errorf("x25519 key: %w", err)
	}
	// clamp like `xray x25519` so both sides agree on the scalar
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("x25519 public key: %w", err)
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(priv), enc.EncodeToString(pub), nil
}

// PublicKeyFor derives the public key from a stored private key.
func PublicKeyFor(private string) (string, error) {
	priv, err := base64.RawURLEncoding.DecodeString(private)
	if err != nil || len(priv) != curve25519.ScalarSize {
		return "", fmt.Errorf("invalid Reality private key")
	}
	pub, err := curve25519.X25519(priv, curve25519.Basepoint)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(pub), nil
}

// ShortIDs returns n random Reality short IDs of 8 bytes in hex.
func ShortIDs(n int) []string {
	out := make([]string, n)
	for i := range out {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			panic(err)
		}
		out[i] = hex.EncodeToString(b)
	}
	return out
}
