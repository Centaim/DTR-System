// Package config reads settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	JWTSecret        []byte
	PhotoDir         string
	MaxPhotoMB       int
	MinGapSecs       int // blocks duplicate taps; a setting, not a constant
	RequestDays      int // how far back a request may be dated
	HabitualLateDays int // late days in a month that flag habitual tardiness
	PINMaxAttempts   int // wrong PINs in a row before the account locks
	PINLockMinutes   int
	CORSOrigins      string
	SeedDemo         bool
}

func Load() (Config, error) {
	c := Config{
		Addr:        env("ADDR", ":8080"),
		DatabaseURL: env("DATABASE_URL", "postgres://dtr:dtr@localhost:5433/dtr?sslmode=disable"),
		PhotoDir:    env("PHOTO_DIR", "./data/photos"),
		CORSOrigins: env("CORS_ORIGINS", "http://localhost:5173"),
		SeedDemo:    env("SEED_DEMO", "false") == "true",
	}
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return c, fmt.Errorf("JWT_SECRET must be set and at least 32 characters")
	}
	c.JWTSecret = []byte(secret)

	mb, err := strconv.Atoi(env("MAX_PHOTO_MB", "5"))
	if err != nil || mb < 1 {
		return c, fmt.Errorf("MAX_PHOTO_MB must be a positive integer")
	}
	c.MaxPhotoMB = mb
	gap, err := strconv.Atoi(env("MIN_GAP_SECONDS", "60"))
	if err != nil || gap < 0 {
		return c, fmt.Errorf("MIN_GAP_SECONDS must be zero or a positive integer")
	}
	c.MinGapSecs = gap
	days, err := strconv.Atoi(env("REQUEST_WINDOW_DAYS", "30"))
	if err != nil || days < 1 {
		return c, fmt.Errorf("REQUEST_WINDOW_DAYS must be a positive integer")
	}
	c.RequestDays = days
	hab, err := strconv.Atoi(env("HABITUAL_LATE_DAYS", "10"))
	if err != nil || hab < 1 {
		return c, fmt.Errorf("HABITUAL_LATE_DAYS must be a positive integer")
	}
	c.HabitualLateDays = hab
	maxTries, err := strconv.Atoi(env("PIN_MAX_ATTEMPTS", "5"))
	if err != nil || maxTries < 3 {
		return c, fmt.Errorf("PIN_MAX_ATTEMPTS must be 3 or more")
	}
	lock, err := strconv.Atoi(env("PIN_LOCK_MINUTES", "10"))
	if err != nil || lock < 1 {
		return c, fmt.Errorf("PIN_LOCK_MINUTES must be a positive integer")
	}
	c.PINMaxAttempts, c.PINLockMinutes = maxTries, lock
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
