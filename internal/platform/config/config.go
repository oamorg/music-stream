package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	ServiceName         string
	AppEnv              string
	HTTPPort            string
	ShutdownTimeout     time.Duration
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	ManifestURLTTL      time.Duration
	AuthLoginRateLimit  int
	AuthLoginRateWindow time.Duration
	EventRateLimit      int
	EventRateWindow     time.Duration
	WorkerPollInterval time.Duration
	DatabaseURL         string
	RedisAddr           string
	MinIOEndpoint       string
	MinIOAccessKey      string
	MinIOSecretKey      string
	MinIOBucket         string
	MediaBaseURL        string
	LocalMediaRoot      string
	FFmpegBinary        string
	JWTAccessSecret     string
	JWTRefreshSecret    string
}

func Load() Config {
	return Config{
		ServiceName:         getenv("SERVICE_NAME", "music-stream"),
		AppEnv:              getenv("APP_ENV", "dev"),
		HTTPPort:            getenv("HTTP_PORT", "8080"),
		ShutdownTimeout:     getenvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
		AccessTokenTTL:      getenvDuration("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     getenvDuration("REFRESH_TOKEN_TTL", 7*24*time.Hour),
		ManifestURLTTL:      getenvDuration("MANIFEST_URL_TTL", 10*time.Minute),
		AuthLoginRateLimit:  getenvInt("AUTH_LOGIN_RATE_LIMIT", 10),
		AuthLoginRateWindow: getenvDuration("AUTH_LOGIN_RATE_WINDOW", time.Minute),
		EventRateLimit:      getenvInt("EVENT_RATE_LIMIT", 120),
		EventRateWindow:     getenvDuration("EVENT_RATE_WINDOW", time.Minute),
		WorkerPollInterval: getenvDuration("WORKER_POLL_INTERVAL", 5*time.Second),
		DatabaseURL:         getenv("DATABASE_URL", "postgres://music:music@localhost:5432/music?sslmode=disable"),
		RedisAddr:           getenv("REDIS_ADDR", "localhost:6379"),
		MinIOEndpoint:       getenv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:      getenv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:      getenv("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:         getenv("MINIO_BUCKET", "music-stream"),
		MediaBaseURL:        getenv("MEDIA_BASE_URL", "http://localhost:8080/media"),
		LocalMediaRoot:      getenv("LOCAL_MEDIA_ROOT", ".data/media"),
		FFmpegBinary:        getenv("FFMPEG_BINARY", "ffmpeg"),
		JWTAccessSecret:     getenv("JWT_ACCESS_SECRET", "change-me-access-secret"),
		JWTRefreshSecret:    getenv("JWT_REFRESH_SECRET", "change-me-refresh-secret"),
	}
}

func getenv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}

	return fallback
}

func getenvDuration(key string, fallback time.Duration) time.Duration {
	raw := getenv(key, fallback.String())
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getenvInt(key string, fallback int) int {
	raw := getenv(key, "")
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}
