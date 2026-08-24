package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Address, DatabasePath, AdminToken, LogLevel string
	Heartbeat, ShutdownTimeout                  time.Duration
	EventHistory                                int
	MaxBodyBytes                                int64
}

func Load() (Config, error) {
	heartbeat, err := time.ParseDuration(env("CONFIG_CENTER_HEARTBEAT", "20s"))
	if err != nil || heartbeat < time.Second {
		return Config{}, fmt.Errorf("invalid CONFIG_CENTER_HEARTBEAT")
	}
	shutdown, err := time.ParseDuration(env("CONFIG_CENTER_SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return Config{}, fmt.Errorf("invalid CONFIG_CENTER_SHUTDOWN_TIMEOUT")
	}
	history, err := strconv.Atoi(env("CONFIG_CENTER_EVENT_HISTORY", "1000"))
	if err != nil || history < 10 {
		return Config{}, fmt.Errorf("CONFIG_CENTER_EVENT_HISTORY must be at least 10")
	}
	cfg := Config{
		Address: env("CONFIG_CENTER_ADDR", "127.0.0.1:8081"), DatabasePath: env("CONFIG_CENTER_DB", "./data/config-center.db"),
		AdminToken: env("CONFIG_CENTER_ADMIN_TOKEN", "local-admin-token"), LogLevel: env("CONFIG_CENTER_LOG_LEVEL", "info"),
		Heartbeat: heartbeat, ShutdownTimeout: shutdown, EventHistory: history, MaxBodyBytes: 2 << 20,
	}
	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("CONFIG_CENTER_ADMIN_TOKEN is required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
