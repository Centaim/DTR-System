package auth

import (
	"encoding/base32"
	"testing"
	"time"
)

// RFC 6238 appendix B, SHA-1 vectors truncated to six digits.
func TestTOTPVectors(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	for at, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		got, err := TOTPCode(secret, at/30)
		if err != nil || got != want {
			t.Errorf("t=%d: got %s want %s err=%v", at, got, want, err)
		}
	}
}

func TestVerifyTOTPRefusesReplayAndOldCodes(t *testing.T) {
	s, _ := NewTOTPSecret()
	now := time.Unix(1_800_000_000, 0)
	step := now.Unix() / 30
	code, _ := TOTPCode(s, step)
	got, ok := VerifyTOTP(s, code, now, 0)
	if !ok || got != step {
		t.Fatalf("valid code refused")
	}
	if _, ok := VerifyTOTP(s, code, now, got); ok {
		t.Error("the same code worked twice")
	}
	old, _ := TOTPCode(s, step-5)
	if _, ok := VerifyTOTP(s, old, now, 0); ok {
		t.Error("an expired code was accepted")
	}
}

func TestSealSecret(t *testing.T) {
	app := []byte("0123456789abcdef0123456789abcdef")
	sealed, err := SealSecret(app, "SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := OpenSecret(app, sealed); err != nil || got != "SECRET" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if _, err := OpenSecret([]byte("another-secret-another-secret-xx"), sealed); err == nil {
		t.Error("opened with the wrong key")
	}
}

func TestMFATokenIsNotASession(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok, err := IssueMFAToken(secret, 9)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := ParseMFAToken(secret, tok); err != nil || id != 9 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if _, err := ParseToken(secret, tok); err == nil {
		t.Error("an unfinished sign-in was accepted as a session")
	}
}
