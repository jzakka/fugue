package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
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
	S3Endpoint          string
	S3Region            string
	S3Bucket            string
	S3AccessKey         string
	S3SecretKey         string
	S3PublicURL         string

	// Scheduler host token bucket (per scheduler-host-token-bucket spec).
	SchedulerHostDefaultRatePerSec  float64
	SchedulerHostDefaultBurst       int
	SchedulerHostTokenBucketEnabled bool

	// Pioneer snapshot storage (per pioneer-snapshot-storage spec).
	// PioneerSnapshotEnabled is the operational toggle: when false, no
	// snapshot uploads are performed (spec: "Scenario: 비활성화 시 업로드 스킵").
	// PioneerSnapshotBucket selects the destination bucket; defaults to
	// the existing media bucket so the snapshots/ prefix can co-exist.
	PioneerSnapshotEnabled bool
	PioneerSnapshotBucket  string
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

	// TODO: Discord를 다시 필수로 변경할 것 (OAuth 앱 등록 후)
	discordID := os.Getenv("DISCORD_CLIENT_ID")
	discordSecret := os.Getenv("DISCORD_CLIENT_SECRET")

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
		S3Endpoint:          envOrDefault("S3_ENDPOINT", "http://localhost:9000"),
		S3Region:            envOrDefault("S3_REGION", "us-east-1"),
		S3Bucket:            envOrDefault("S3_BUCKET", "fugue-media"),
		S3AccessKey:         envOrDefault("S3_ACCESS_KEY", "fugue"),
		S3SecretKey:         envOrDefault("S3_SECRET_KEY", "fuguedev123"),
		S3PublicURL:         envOrDefault("S3_PUBLIC_URL", "http://localhost:9000/fugue-media"),

		SchedulerHostDefaultRatePerSec:  envFloat("SCHEDULER_HOST_DEFAULT_RATE_PER_SEC", 1.0),
		SchedulerHostDefaultBurst:       envInt("SCHEDULER_HOST_DEFAULT_BURST", 5),
		SchedulerHostTokenBucketEnabled: envBool("SCHEDULER_HOST_TOKEN_BUCKET_ENABLED", true),

		// Default off; staging/prod enable explicitly during rollout.
		PioneerSnapshotEnabled: envBool("PIONEER_SNAPSHOT_ENABLED", false),
		PioneerSnapshotBucket:  envOrDefault("PIONEER_SNAPSHOT_BUCKET", envOrDefault("S3_BUCKET", "fugue-media")),
	}, nil
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
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
