package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
)

const version byte = 1

type Key struct {
	aead cipher.AEAD
	raw  []byte
}

func ParseKey(s string) (*Key, error) {
	s = strings.TrimSpace(s)
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
	}
	if err != nil || len(raw) != 32 {
		return nil, errors.New("the master key must be 32 bytes, base64-encoded")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Key{aead: aead, raw: raw}, nil
}

func (k *Key) Seal(plaintext, context []byte) []byte {
	nonce := make([]byte, k.aead.NonceSize())
	_, _ = rand.Read(nonce)
	out := append([]byte{version}, nonce...)
	return k.aead.Seal(out, nonce, plaintext, context)
}

func (k *Key) Open(sealed, context []byte) ([]byte, error) {
	n := k.aead.NonceSize()
	if len(sealed) < 1+n || sealed[0] != version {
		return nil, errors.New("secret: unknown format")
	}
	return k.aead.Open(nil, sealed[1:1+n], sealed[1+n:], context)
}

// Derive returns a 32-byte key for one purpose, named by info, derived from the master key with
// HKDF-SHA256, so a key used for hashing never doubles as the encryption key.
func (k *Key) Derive(info string) []byte {
	out, err := hkdf.Key(sha256.New, k.raw, nil, info, 32)
	if err != nil {
		panic(err)
	}
	return out
}
