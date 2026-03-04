package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL              string
	ServerPort               string
	HealthPort               string
	LogLevel                 string
	LeaseDurationSeconds     int
	ClaimBatchSize           int
	WorkerID                 string
	HeartbeatIntervalSeconds int
	ReaperIntervalSeconds    int
	PublisherIntervalSeconds int
	PublisherBatchSize       int
	DBMaxConns               int
	DBMinConns               int
	DBMaxConnLifetime        time.Duration
	DBMaxConnIdleTime        time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:              getEnv("DATABASE_URL", "postgres://narayana:narayana@localhost:5432/narayana?sslmode=disable"),
		ServerPort:               getEnv("SERVER_PORT", "8080"),
		HealthPort:               getEnv("HEALTH_PORT", "8081"),
		LogLevel:                 getEnv("LOG_LEVEL", "debug"),
		LeaseDurationSeconds:     getEnvInt("LEASE_DURATION_SECONDS", 30),
		ClaimBatchSize:           getEnvInt("CLAIM_BATCH_SIZE", 10),
		WorkerID:                 getEnv("WORKER_ID", "worker-1"),
		HeartbeatIntervalSeconds: getEnvInt("HEARTBEAT_INTERVAL_SECONDS", 10),
		ReaperIntervalSeconds:    getEnvInt("REAPER_INTERVAL_SECONDS", 10),
		PublisherIntervalSeconds: getEnvInt("PUBLISHER_INTERVAL_SECONDS", 2),
		PublisherBatchSize:       getEnvInt("PUBLISHER_BATCH_SIZE", 50),
		DBMaxConns:               getEnvInt("DB_MAX_CONNS", 20),
		DBMinConns:               getEnvInt("DB_MIN_CONNS", 5),
		DBMaxConnLifetime:        getEnvDuration("DB_MAX_CONN_LIFETIME", 30*time.Minute),
		DBMaxConnIdleTime:        getEnvDuration("DB_MAX_CONN_IDLE_TIME", 5*time.Minute),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	parsed, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return parsed
}
