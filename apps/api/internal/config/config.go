package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port                string
	DatabaseURL         string
	RedisURL            string
	JWTSecret           []byte
	OAuthCallbackBase   string
	FrontendURL         string
	GoogleClientID      string
	GoogleClientSecret  string
	DiscordClientID     string
	DiscordClientSecret string
	TwitterClientID     string
	TwitterClientSecret string
}

func Load() (*Config, error) {
	jwtSecretB64 := os.Getenv("JWT_SECRET")
	if jwtSecretB64 == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	jwtSecret, err := base64.StdEncoding.DecodeString(jwtSecretB64)
	if err != nil {
		return nil, fmt.Errorf("JWT_SECRET must be base64-encoded: %w", err)
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}

	callbackBase := os.Getenv("OAUTH_CALLBACK_BASE_URL")
	if callbackBase == "" {
		return nil, fmt.Errorf("OAUTH_CALLBACK_BASE_URL is required")
	}

	googleID := os.Getenv("GOOGLE_CLIENT_ID")
	googleSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	if googleID == "" || googleSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required")
	}

	discordID := os.Getenv("DISCORD_CLIENT_ID")
	discordSecret := os.Getenv("DISCORD_CLIENT_SECRET")
	if discordID == "" || discordSecret == "" {
		return nil, fmt.Errorf("DISCORD_CLIENT_ID and DISCORD_CLIENT_SECRET are required")
	}

	return &Config{
		Port:                envOrDefault("PORT", "8080"),
		DatabaseURL:         envOrDefault("DATABASE_URL", "postgres://fugue:fugue@localhost:5432/fugue?sslmode=disable"),
		RedisURL:            envOrDefault("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:           jwtSecret,
		OAuthCallbackBase:   strings.TrimRight(callbackBase, "/"),
		FrontendURL:         envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		GoogleClientID:      googleID,
		GoogleClientSecret:  googleSecret,
		DiscordClientID:     discordID,
		DiscordClientSecret: discordSecret,
		TwitterClientID:     os.Getenv("TWITTER_CLIENT_ID"),
		TwitterClientSecret: os.Getenv("TWITTER_CLIENT_SECRET"),
	}, nil
}

func (c *Config) IsDevMode() bool {
	return strings.HasPrefix(c.OAuthCallbackBase, "http://")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
