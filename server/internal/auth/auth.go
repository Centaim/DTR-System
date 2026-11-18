// Package auth handles PIN login and signed access tokens.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

const tokenTTL = 15 * time.Minute

// HashPIN returns an argon2id hash in the form argon2id$salt$hash (base64).
func HashPIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(pin), salt, 2, 64*1024, 2, 32)
	return fmt.Sprintf("argon2id$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(h)), nil
}

func VerifyPIN(pin, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[1])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[2])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, 2, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}

type Claims struct {
	EmployeeID int64  `json:"eid"`
	Kind       string `json:"k,omitempty"` // "" for a normal session, "kiosk" for a one-shot kiosk clock-in
	KioskID    int64  `json:"kid,omitempty"`
	jwt.RegisteredClaims
}

func IssueToken(secret []byte, employeeID int64) (string, error) {
	now := time.Now()
	c := Claims{
		EmployeeID: employeeID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
}

func ParseToken(secret []byte, token string) (int64, error) {
	var c Claims
	t, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		return secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid || c.Kind != "" {
		return 0, errors.New("invalid token")
	}
	return c.EmployeeID, nil
}

const kioskTokenTTL = 2 * time.Minute

// IssueKioskToken lets one verified employee take one photo at one kiosk.
// It cannot be used as an ordinary session.
func IssueKioskToken(secret []byte, employeeID, kioskID int64) (string, error) {
	now := time.Now()
	c := Claims{
		EmployeeID: employeeID, Kind: "kiosk", KioskID: kioskID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(kioskTokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
}

// IssueMFAToken proves the PIN was right while the second factor is still pending.
func IssueMFAToken(secret []byte, employeeID int64) (string, error) {
	now := time.Now()
	c := Claims{EmployeeID: employeeID, Kind: "mfa", RegisteredClaims: jwt.RegisteredClaims{
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(secret)
}

func ParseMFAToken(secret []byte, token string) (int64, error) {
	var c Claims
	t, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid || c.Kind != "mfa" {
		return 0, errors.New("invalid token")
	}
	return c.EmployeeID, nil
}

func ParseKioskToken(secret []byte, token string) (employeeID, kioskID int64, err error) {
	var c Claims
	t, perr := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) { return secret, nil },
		jwt.WithValidMethods([]string{"HS256"}))
	if perr != nil || !t.Valid || c.Kind != "kiosk" {
		return 0, 0, errors.New("invalid kiosk token")
	}
	return c.EmployeeID, c.KioskID, nil
}

// ValidatePIN enforces the PIN policy: 6 to 12 digits, nothing trivially guessable.
func ValidatePIN(pin string) error {
	if len(pin) < 6 || len(pin) > 12 {
		return errors.New("PIN must be 6 to 12 digits.")
	}
	for _, r := range pin {
		if r < '0' || r > '9' {
			return errors.New("PIN must use digits only.")
		}
	}
	same, asc, desc := true, true, true
	for i := 1; i < len(pin); i++ {
		if pin[i] != pin[0] {
			same = false
		}
		if pin[i] != pin[i-1]+1 {
			asc = false
		}
		if pin[i] != pin[i-1]-1 {
			desc = false
		}
	}
	if same || asc || desc {
		return errors.New("PIN is too easy to guess. Avoid repeated or counting digits.")
	}
	return nil
}

// TempPIN returns a random 8-digit PIN for first sign-in. The owner must change it.
func TempPIN() (string, error) {
	for {
		n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
		if err != nil {
			return "", err
		}
		pin := fmt.Sprintf("%08d", n.Int64())
		if ValidatePIN(pin) == nil {
			return pin, nil
		}
	}
}
