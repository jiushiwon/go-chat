// Package config 加载 .env 与环境变量，提供全局只读 Config。
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort    string
	AppMode    string
	AppLogLvl  string

	DBPath        string
	DBBusyTimeout int

	TokenTTLHrs       int
	BcryptCost        int
	RateLimitPerMin   int

	MsgRetentionDays    int
	CleanupIntervalMin  int
}

func Load() *Config {
	// 尝试加载 .env（不存在不报错）
	_ = godotenv.Load()

	return &Config{
		AppPort:   getEnv("APP_PORT", "8088"),
		AppMode:   getEnv("APP_MODE", "release"),
		AppLogLvl: getEnv("APP_LOG_LEVEL", "info"),

		DBPath:        getEnv("DB_PATH", "./data/go-ws-sqlite.db"),
		DBBusyTimeout: getEnvInt("DB_BUSY_TIMEOUT_MS", 5000),

		TokenTTLHrs:     getEnvInt("TOKEN_TTL_HOURS", 720),
		BcryptCost:      getEnvInt("PASSWORD_BCRYPT_COST", 10),
		RateLimitPerMin: getEnvInt("RATE_LIMIT_PER_MIN", 600),

		MsgRetentionDays:   getEnvInt("MESSAGE_RETENTION_DAYS", 7),
		CleanupIntervalMin: getEnvInt("CLEANUP_INTERVAL_MINUTES", 60),
	}
}

// LogLevel 把字符串映射到 slog.Level。
func (c *Config) LogLevel() slog.Level {
	switch strings.ToLower(c.AppLogLvl) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}