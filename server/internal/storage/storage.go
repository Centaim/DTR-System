// Package storage keeps attendance photos. Phase 0 uses local disk behind an
// interface so S3-compatible storage (MinIO) can replace it without API changes.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Store interface {
	Put(ctx context.Context, key string, data []byte) (sha256Hex string, err error)
	Get(ctx context.Context, key string) ([]byte, error)
}

type Disk struct{ root string }

func NewDisk(root string) (*Disk, error) {
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &Disk{root: root}, nil
}

func (d *Disk) path(key string) (string, error) {
	clean := filepath.Clean(key)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid photo key")
	}
	return filepath.Join(d.root, clean), nil
}

func (d *Disk) Put(_ context.Context, key string, data []byte) (string, error) {
	p, err := d.path(key)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, data, 0o640); err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (d *Disk) Get(_ context.Context, key string) ([]byte, error) {
	p, err := d.path(key)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}
