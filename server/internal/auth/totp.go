package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 authenticator apps use HMAC-SHA1
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const totpStep = 30

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random secret in the base32 form authenticator apps expect.
func NewTOTPSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return b32.EncodeToString(b), nil
}

// TOTPCode computes the 6-digit code for one time step (RFC 6238).
func TOTPCode(secret string, step int64) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", v%1000000), nil
}

// VerifyTOTP accepts the current code or its neighbours, to allow for clock
// drift, and returns the step that matched. Steps at or before lastStep are
// refused so a code cannot be used twice.
func VerifyTOTP(secret, code string, now time.Time, lastStep int64) (int64, bool) {
	cur := now.Unix() / totpStep
	for _, s := range []int64{cur - 1, cur, cur + 1} {
		if s <= lastStep {
			continue
		}
		want, err := TOTPCode(secret, s)
		if err == nil && hmac.Equal([]byte(want), []byte(strings.TrimSpace(code))) {
			return s, true
		}
	}
	return 0, false
}

// TOTPURL is what the QR code holds.
func TOTPURL(issuer, account, secret string) string {
	v := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + v.Encode()
}

func sealKey(appSecret []byte) []byte {
	k := sha256.Sum256(append([]byte("dtr-mfa-seal:"), appSecret...))
	return k[:]
}

// SealSecret encrypts an MFA secret for storage.
func SealSecret(appSecret []byte, plain string) (string, error) {
	block, err := aes.NewCipher(sealKey(appSecret))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func OpenSecret(appSecret []byte, sealed string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(sealKey(appSecret))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < aead.NonceSize() {
		return "", errors.New("sealed secret is damaged")
	}
	plain, err := aead.Open(nil, raw[:aead.NonceSize()], raw[aead.NonceSize():], nil)
	return string(plain), err
}

// RandomToken returns a URL-safe random token.
func RandomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is for high-entropy tokens only; PINs use argon2id.
func HashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return base64.RawStdEncoding.EncodeToString(sum[:])
}
