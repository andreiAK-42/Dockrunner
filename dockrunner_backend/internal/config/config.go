package config

import (
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env     string
	Port    string
	EnvFile bool

	// Logging
	LogDir string

	// HTTP
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Postgres (main OLTP)
	PostgresURL         string
	PostgresMaxConns    int32
	PostgresMinConns    int32
	PostgresMaxConnLife time.Duration
	PostgresMaxConnIdle time.Duration
	PostgresHealthCheck time.Duration

	// InfluxDB (metrics / time-series)
	InfluxURL     string
	InfluxToken   string
	InfluxOrg     string
	InfluxBucket  string
	InfluxTimeout time.Duration

	// RabbitMQ (async tasks)
	RabbitURL           string
	RabbitTLSSkipVerify bool

	// Auth / JWT
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration
	BcryptCost    int
	AgentToken    string
	AdminLogin    string
	AdminPassword string

	// CORS
	CORSAllowedOrigins []string
	CORSAllowedMethods []string
	CORSAllowedHeaders []string

	// Rate limiter
	RateLimitRPS   int
	RateLimitBurst int

	// User host ports allowed range
	PortsMin int
	PortsMax int

	// Managed databases (провижининг пользователей СУБД).
	// Хосты/порты для строк подключения, admin-доступ — для CREATE USER.
	DBPostgresHost     string
	DBPostgresPort     int
	DBPostgresName     string
	DBPostgresAdminURL string
	DBMongoHost        string
	DBMongoPort        int
	DBMongoName        string
	DBMongoAdminURL    string
	DBRedisHost        string
	DBRedisPort        int
	DBRedisPassword    string
}

func Load() (*Config, error) {
	// Load .env if exists, ignore error
	envErr := godotenv.Load()
	if envErr != nil {
		envErr = godotenv.Load(".env")
	}

	cfg := &Config{
		Env:     getEnv("APP_ENV", "development"),
		Port:    getEnv("PORT", "8080"),
		EnvFile: envErr == nil,

		LogDir: getEnv("LOG_DIR", "logs"),

		ReadTimeout:  getDuration("HTTP_READ_TIMEOUT", 15*time.Second),
		WriteTimeout: getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:  getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),

		PostgresURL:         getEnv("POSTGRES_URL", "postgres://dockrunner:dockrunner@localhost:5432/dockrunner?sslmode=disable"),
		PostgresMaxConns:    getInt32("POSTGRES_MAX_CONNS", 20),
		PostgresMinConns:    getInt32("POSTGRES_MIN_CONNS", 2),
		PostgresMaxConnLife: getDuration("POSTGRES_MAX_CONN_LIFETIME", 30*time.Minute),
		PostgresMaxConnIdle: getDuration("POSTGRES_MAX_CONN_IDLE", 5*time.Minute),
		PostgresHealthCheck: getDuration("POSTGRES_HEALTH_CHECK_PERIOD", 1*time.Minute),

		InfluxURL:     getEnv("INFLUX_URL", "http://localhost:8086"),
		InfluxToken:   getEnv("INFLUX_TOKEN", "dockrunner-influx-token"),
		InfluxOrg:     getEnv("INFLUX_ORG", "dockrunner"),
		InfluxBucket:  getEnv("INFLUX_BUCKET", "metrics"),
		InfluxTimeout: getDuration("INFLUX_TIMEOUT", 5*time.Second),

		RabbitURL:           getEnv("RABBITMQ_URL", "amqp://dockrunner:dockrunner@localhost:5672/"),
		RabbitTLSSkipVerify: getBool("RABBITMQ_TLS_SKIP_VERIFY", false),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-super-secret-key-32-chars"),
		JWTAccessTTL:  getDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: getDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		BcryptCost:    getInt("BCRYPT_COST", 10),
		AgentToken:    getEnv("AGENT_TOKEN", ""),
		AdminLogin:    getEnv("ADMIN_LOGIN", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin123"),

		CORSAllowedOrigins: getSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173"}),
		CORSAllowedMethods: getSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}),
		CORSAllowedHeaders: getSlice("CORS_ALLOWED_HEADERS", []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"}),

		RateLimitRPS:   getInt("RATE_LIMIT_RPS", 100),
		RateLimitBurst: getInt("RATE_LIMIT_BURST", 200),

		PortsMin: getInt("PORTS_MIN", 3000),
		PortsMax: getInt("PORTS_MAX", 4000),

		DBPostgresHost:     getEnv("DB_POSTGRES_HOST", "pg.dockrunner.local"),
		DBPostgresPort:     getInt("DB_POSTGRES_PORT", 5432),
		DBPostgresName:     getEnv("DB_POSTGRES_NAME", "appdb"),
		DBPostgresAdminURL: getEnv("DB_POSTGRES_ADMIN_URL", ""),
		DBMongoHost:        getEnv("DB_MONGO_HOST", "mongo.dockrunner.local"),
		DBMongoPort:        getInt("DB_MONGO_PORT", 27017),
		DBMongoName:        getEnv("DB_MONGO_NAME", "appdb"),
		DBMongoAdminURL:    getEnv("DB_MONGO_ADMIN_URL", ""),
		DBRedisHost:        getEnv("DB_REDIS_HOST", "redis.dockrunner.local"),
		DBRedisPort:        getInt("DB_REDIS_PORT", 6379),
		DBRedisPassword:    getEnv("DB_REDIS_PASSWORD", ""),
	}

	if cfg.PortsMin < 1 {
		cfg.PortsMin = 1
	}
	if cfg.PortsMax > 65535 {
		cfg.PortsMax = 65535
	}
	if cfg.PortsMin > cfg.PortsMax {
		cfg.PortsMax = cfg.PortsMin
	}

	return cfg, nil
}

func (c *Config) IsProduction() bool  { return c.Env == "production" }
func (c *Config) IsDevelopment() bool { return c.Env == "development" }

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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
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

func getInt32(key string, fallback int32) int32 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(i)
		}
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

func getSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return fallback
}
