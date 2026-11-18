package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestPINHashAndVerify(t *testing.T) {
	h, err := HashPIN("123456")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPIN("123456", h) {
		t.Error("correct PIN rejected")
	}
	if VerifyPIN("654321", h) {
		t.Error("wrong PIN accepted")
	}
	if VerifyPIN("123456", "garbage") {
		t.Error("malformed hash accepted")
	}
	h2, _ := HashPIN("123456")
	if h == h2 {
		t.Error("hashes must be salted")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok, err := IssueToken(secret, 42)
	if err != nil {
		t.Fatal(err)
	}
	id, err := ParseToken(secret, tok)
	if err != nil || id != 42 {
		t.Fatalf("got id=%d err=%v", id, err)
	}
	if _, err := ParseToken([]byte("a-different-secret-a-different-secret"), tok); err == nil {
		t.Error("token signed with another secret was accepted")
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	c := Claims{
		EmployeeID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
	if _, err := ParseToken(secret, tok); err == nil {
		t.Error("expired token accepted")
	}
}

func TestNoneAlgorithmRejected(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{EmployeeID: 1}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := ParseToken(secret, tok); err == nil {
		t.Error("unsigned token accepted")
	}
}
