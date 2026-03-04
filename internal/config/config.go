package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL              string
	ServerPort               string
	LogLevel                 string
	LeaseDurationSeconds     int
	ClaimBatchSize           int
	WorkerID                 string
	HeartbeatIntervalSeconds int
	ReaperIntervalSeconds    int
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL:              getEnv("DATABASE_URL", "postgres://narayana:narayana@localhost:5432/narayana?sslmode=disable"),
		ServerPort:               getEnv("SERVER_PORT", "8080"),
		LogLevel:                 getEnv("LOG_LEVEL", "debug"),
		LeaseDurationSeconds:     getEnvInt("LEASE_DURATION_SECONDS", 30),
		ClaimBatchSize:           getEnvInt("CLAIM_BATCH_SIZE", 10),
		WorkerID:                 getEnv("WORKER_ID", "worker-1"),
		HeartbeatIntervalSeconds: getEnvInt("HEARTBEAT_INTERVAL_SECONDS", 10),
		ReaperIntervalSeconds:    getEnvInt("REAPER_INTERVAL_SECONDS", 10),
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
