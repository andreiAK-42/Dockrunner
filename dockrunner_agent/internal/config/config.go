package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const Version = "0.1.0"

type Config struct {
	AgentName        string
	AgentDescription string
	AgentPublicURL   string
	CentralURL       string
	Token            string

	RabbitURL           string
	RabbitTLSSkipVerify bool

	SQLitePath    string
	Workspace     string
	HostWorkspace string

	Port      string
	Heartbeat time.Duration
	AgentIP   string

	VictoriaMetricsURL string
	VictoriaLogsURL    string
	VictoriaToken      string
}

// Load загружает конфигурацию из .env и переменных окружения.
func Load() (*Config, error) {
	// Загружаем .env, если файл существует.
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("Ошибка загрузки конфигурации .env: %w", err)
	}

	cfg := Config{}

	//Функция для загрузки переменных
	getEnv := func(key string) string {
		return os.Getenv(key)
	}

	cfg.AgentName = getEnv("AGENT_NAME")

	cfg.AgentDescription = getEnv("AGENT_DESCRIPTION")
	cfg.AgentPublicURL = getEnv("AGENT_PUBLIC_URL")
	cfg.AgentIP = getEnv("AGENT_IP")

	cfg.CentralURL = getEnv("CENTRAL_URL")
	cfg.Token = getEnv("AGENT_TOKEN")

	cfg.RabbitURL = getEnv("RABBITMQ_URL")
	cfg.RabbitTLSSkipVerify, _ = strconv.ParseBool(getEnv("RABBITMQ_TLS_SKIP_VERIFY"))

	cfg.SQLitePath = getEnv("SQLITE_PATH")
	cfg.Workspace = getEnv("WORKSPACE_DIR")
	cfg.HostWorkspace = getEnv("HOST_WORKSPACE_DIR")

	cfg.Port = getEnv("PORT")
	// HEARTBEAT_INTERVAL принимаем и как "15s", и как число секунд "15".
	if raw := getEnv("HEARTBEAT_INTERVAL"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			cfg.Heartbeat = d
		} else if seconds, err := strconv.Atoi(raw); err == nil {
			cfg.Heartbeat = time.Duration(seconds) * time.Second
		}
	}

	cfg.VictoriaMetricsURL = getEnv("VICTORIA_METRICS_URL")
	cfg.VictoriaLogsURL = getEnv("VICTORIA_LOGS_URL")
	cfg.VictoriaToken = getEnv("VICTORIA_TOKEN")

	return &cfg, nil
}

func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "invalid-url"
	}

	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "***")
	}

	return u.String()
}
