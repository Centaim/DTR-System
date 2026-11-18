package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedRoundTripLeavesNoPlaintext(t *testing.T) {
	root := t.TempDir()
	d, _ := NewDisk(root)
	key := bytes.Repeat([]byte{7}, 32)
	e, err := NewEncrypted(d, key)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	secret := []byte("recognisable-photo-bytes")
	if _, err := e.Put(ctx, "a/b.img", secret); err != nil {
		t.Fatal(err)
	}
	onDisk, _ := os.ReadFile(filepath.Join(root, "a/b.img"))
	if bytes.Contains(onDisk, secret) {
		t.Error("photo is readable on disk")
	}
	got, err := e.Get(ctx, "a/b.img")
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("got %q err=%v", got, err)
	}
	other, _ := NewEncrypted(d, bytes.Repeat([]byte{8}, 32))
	if _, err := other.Get(ctx, "a/b.img"); err == nil {
		t.Error("wrong key opened the photo")
	}
	onDisk[len(onDisk)-1] ^= 1
	_ = os.WriteFile(filepath.Join(root, "a/b.img"), onDisk, 0o640)
	if _, err := e.Get(ctx, "a/b.img"); err == nil {
		t.Error("tampered photo was accepted")
	}
}

func TestEncryptedReadsOlderPlainPhotos(t *testing.T) {
	d, _ := NewDisk(t.TempDir())
	e, _ := NewEncrypted(d, bytes.Repeat([]byte{7}, 32))
	ctx := context.Background()
	_, _ = d.Put(ctx, "old.img", []byte("plain"))
	if got, err := e.Get(ctx, "old.img"); err != nil || string(got) != "plain" {
		t.Fatalf("got %q err=%v", got, err)
	}
}
