package storage

import (
	"context"
	"testing"
)

func TestDiskRoundTripAndHash(t *testing.T) {
	d, err := NewDisk(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sum, err := d.Put(ctx, "2026/11/18/1/a.img", []byte("photo-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if len(sum) != 64 {
		t.Errorf("expected sha256 hex, got %q", sum)
	}
	got, err := d.Get(ctx, "2026/11/18/1/a.img")
	if err != nil || string(got) != "photo-bytes" {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestDiskRejectsPathEscape(t *testing.T) {
	d, _ := NewDisk(t.TempDir())
	for _, key := range []string{"../evil", "/etc/passwd", "..", "a/../../evil"} {
		if _, err := d.Put(context.Background(), key, []byte("x")); err == nil {
			t.Errorf("key %q was accepted", key)
		}
	}
}
