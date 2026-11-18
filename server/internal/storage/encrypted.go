package storage

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
)

var magic = []byte("DTRE1")

// Encrypted seals every photo with AES-256-GCM before it reaches the inner store.
// Files written before encryption was switched on are still readable.
type Encrypted struct {
	inner Store
	aead  cipher.AEAD
}

func NewEncrypted(inner Store, key []byte) (*Encrypted, error) {
	if len(key) != 32 {
		return nil, errors.New("photo key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encrypted{inner: inner, aead: aead}, nil
}

// Put returns the hash of the original photo, so audit hashes do not depend on encryption.
func (e *Encrypted) Put(ctx context.Context, key string, data []byte) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := append(append([]byte{}, magic...), nonce...)
	out = e.aead.Seal(out, nonce, data, []byte(key))
	if _, err := e.inner.Put(ctx, key, out); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (e *Encrypted) Get(ctx context.Context, key string) ([]byte, error) {
	raw, err := e.inner.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if !bytes.HasPrefix(raw, magic) {
		return raw, nil
	}
	body := raw[len(magic):]
	if len(body) < e.aead.NonceSize() {
		return nil, fmt.Errorf("photo %s is damaged", key)
	}
	nonce, sealed := body[:e.aead.NonceSize()], body[e.aead.NonceSize():]
	return e.aead.Open(nil, nonce, sealed, []byte(key))
}

func (e *Encrypted) Delete(ctx context.Context, key string) error { return e.inner.Delete(ctx, key) }
