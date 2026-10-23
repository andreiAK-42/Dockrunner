package config

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const Version = "0.1.0"

type Config struct {
	EnvFile             bool
	AgentID             string
	AgentName           string
	AgentDescription    string
	AgentCity           string
	AgentLat            float64
	AgentLng            float64
	AgentPublicURL      string
	// Явный внешний IP сервера (обязателен, если агент в docker:
	// из контейнера хостовый IP автоопределением не достать).
	AgentIP             string
	CentralURL          string
	Token               string
	RabbitURL           string
	RabbitTLSSkipVerify bool
	SQLitePath          string
	Workspace           string
	// Тот же workspace глазами docker-демона (хоста). Пусто = как Workspace.
	HostWorkspace       string
	Port                string
	Heartbeat           time.Duration
}

func loadEnvFile() bool {
	if err := godotenv.Load(); err == nil {
		return true
	}
	if err := godotenv.Load(".env"); err == nil {
		return true
	}
	if exe, err := os.Executable(); err == nil {
		if err := godotenv.Load(filepath.Join(filepath.Dir(exe), ".env")); err == nil {
			return true
		}
	}
	return false
}

func Load() Config {
	envFile := loadEnvFile()
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "worker-01"
	}
	return Config{
		EnvFile:             envFile,
		AgentID:             getEnv("AGENT_ID", hostname),
		AgentName:           getEnv("AGENT_NAME", hostname),
		AgentDescription:    getEnv("AGENT_DESCRIPTION", ""),
		AgentCity:           getEnv("AGENT_CITY", ""),
		AgentLat:            getFloat("AGENT_LAT", 0),
		AgentLng:            getFloat("AGENT_LNG", 0),
		AgentPublicURL:      getEnv("AGENT_PUBLIC_URL", ""),
		AgentIP:             getEnv("AGENT_IP", ""),
		CentralURL:          getEnv("CENTRAL_URL", "http://localhost:8080"),
		Token:               getEnv("AGENT_TOKEN", ""),
		RabbitURL:           getEnv("RABBITMQ_URL", "amqp://dockrunner:dockrunner@localhost:5672/"),
		RabbitTLSSkipVerify: getBool("RABBITMQ_TLS_SKIP_VERIFY", false),
		SQLitePath:          getEnv("SQLITE_PATH", "./data/agent.db"),
		Workspace:           getEnv("WORKSPACE_DIR", "./workspace"),
		HostWorkspace:       getEnv("HOST_WORKSPACE_DIR", ""),
		Port:                getEnv("PORT", "8090"),
		Heartbeat:           getDuration("HEARTBEAT_INTERVAL", 15*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
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
