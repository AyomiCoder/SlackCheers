package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Server    ServerConfig
	DB        DBConfig
	Scheduler SchedulerConfig
	Slack     SlackConfig
}

type AppConfig struct {
	Name        string
	Environment string
}

type ServerConfig struct {
	Port string
}

type DBConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	MigrationsDir   string
	AutoMigrate     bool
}

type SchedulerConfig struct {
	Enabled      bool
	PollInterval time.Duration
}

type SlackConfig struct {
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	BotScopes     string
	UserScopes    string
	BotToken      string
	SigningSecret string
}

func Load() (Config, error) {
	// Load .env file if it exists (ignore error for production where env vars are set directly)
	_ = godotenv.Load()

	cfg := Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME", "slackcheers"),
			Environment: getEnv("APP_ENV", "development"),
		},
		Server: ServerConfig{
			Port: getEnv("APP_PORT", "9060"),
		},
		DB: DBConfig{
			URL:             strings.TrimSpace(os.Getenv("DATABASE_URL")),
			MaxOpenConns:    getInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getInt("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: getDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			MigrationsDir:   getEnv("MIGRATIONS_DIR", "db/migrations"),
			AutoMigrate:     getBool("MIGRATIONS_AUTO_APPLY", true),
		},
		Scheduler: SchedulerConfig{
			Enabled:      getBool("SCHEDULER_ENABLED", true),
			PollInterval: getDuration("SCHEDULER_POLL_INTERVAL", time.Minute),
		},
		Slack: SlackConfig{
			ClientID:      strings.TrimSpace(os.Getenv("SLACK_CLIENT_ID")),
			ClientSecret:  strings.TrimSpace(os.Getenv("SLACK_CLIENT_SECRET")),
			RedirectURL:   strings.TrimSpace(os.Getenv("SLACK_REDIRECT_URL")),
			BotScopes:     getEnv("SLACK_BOT_SCOPES", "chat:write,channels:read,users:read"),
			UserScopes:    strings.TrimSpace(os.Getenv("SLACK_USER_SCOPES")),
			BotToken:      strings.TrimSpace(os.Getenv("SLACK_BOT_TOKEN")),
			SigningSecret: strings.TrimSpace(os.Getenv("SLACK_SIGNING_SECRET")),
		},
	}

	if cfg.DB.URL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func getInt(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getBool(key string, fallback bool) bool {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return parsed
}

func getDuration(key string, fallback time.Duration) time.Duration {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(val)
	if err != nil {
		return fallback
	}
	return parsed
}
