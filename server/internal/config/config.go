// Package config reads settings from the environment.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Addr        string
	DatabaseURL string
	JWTSecret   []byte
	PhotoDir    string
	MaxPhotoMB  int
	MinGapSecs  int // blocks duplicate taps; a setting, not a constant
	RequestDays int // how far back a request may be dated
	CORSOrigins string
	SeedDemo    bool
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
	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
