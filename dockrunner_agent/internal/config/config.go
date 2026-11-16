package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"net/url"

	"gopkg.in/yaml.v3"
)

const Version = "0.1.0"

const configFile = "config.yaml"

// Config — YAML-конфиг агента. Поля City/Lat/Lng/IP в файл не пишутся:
// город и координаты определяются при старте, IP — из geo или локального интерфейса.
// AgentIP можно указать в конфиге для принудительного задания IP агента.
type Config struct {
	AgentName           string        `yaml:"AgentName"`
	AgentDescription    string        `yaml:"AgentDescription"`
	AgentPublicURL      string        `yaml:"AgentPublicURL"`
	CentralURL          string        `yaml:"CentralURL"`
	Token               string        `yaml:"Token"`
	RabbitURL           string        `yaml:"RabbitURL"`
	RabbitTLSSkipVerify bool          `yaml:"RabbitTLSSkipVerify"`
	SQLitePath          string        `yaml:"SQLitePath"`
	Workspace           string        `yaml:"Workspace"`
	HostWorkspace       string        `yaml:"HostWorkspace"`
	Port                string        `yaml:"Port"`
	Heartbeat           time.Duration `yaml:"-"`
	HeartbeatRaw        yaml.Node     `yaml:"Heartbeat"`
	AgentIP             string        `yaml:"AgentIP"`

	// Victoria: метрики напрямую в VictoriaMetrics, логи — в VictoriaLogs.
	// Пустой URL — отправка отключена. Token — Bearer для vmauth (можно пусто
	// в доверенной сети).
	VictoriaMetricsURL string `yaml:"VictoriaMetricsURL"`
	VictoriaLogsURL    string `yaml:"VictoriaLogsURL"`
	VictoriaToken      string `yaml:"VictoriaToken"`

	City string  `yaml:"-"`
	Lat  float64 `yaml:"-"`
	Lng  float64 `yaml:"-"`
	IP   string  `yaml:"-"`
}

func Load() (*Config, error) {
	cfg, err := LoadOrCreateConfig()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadOrCreateConfig() (*Config, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := os.WriteFile(configFile, []byte(defaultConfigYAML), 0644); err != nil {
			return nil, fmt.Errorf("создание config.yaml: %w", err)
		}
		fmt.Printf("config created: %s\n", configFile)
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("чтение config.yaml: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("разбор config.yaml: %w", err)
	}
	cfg.normalize()
	cfg.applyEnv()
	return &cfg, nil
}

// applyEnv перекрывает поля конфигурации переменными окружения —
// так контейнеру можно подать готовый env_file вместо правки config.yaml.
// Пустое значение переменной не перекрывает (можно задать только часть).
func (c *Config) applyEnv() {
	override := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	override("AGENT_ID", &c.AgentName)
	override("AGENT_NAME", &c.AgentName)
	override("AGENT_DESCRIPTION", &c.AgentDescription)
	override("AGENT_PUBLIC_URL", &c.AgentPublicURL)
	override("AGENT_IP", &c.AgentIP)
	override("CENTRAL_URL", &c.CentralURL)
	override("AGENT_TOKEN", &c.Token)
	override("RABBITMQ_URL", &c.RabbitURL)
	override("SQLITE_PATH", &c.SQLitePath)
	override("WORKSPACE_DIR", &c.Workspace)
	override("HOST_WORKSPACE_DIR", &c.HostWorkspace)
	override("PORT", &c.Port)
	override("VICTORIA_METRICS_URL", &c.VictoriaMetricsURL)
	override("VICTORIA_LOGS_URL", &c.VictoriaLogsURL)
	override("VICTORIA_TOKEN", &c.VictoriaToken)
	if v := os.Getenv("RABBITMQ_TLS_SKIP_VERIFY"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			c.RabbitTLSSkipVerify = b
		}
	}
	if v := os.Getenv("HEARTBEAT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			c.Heartbeat = d
		}
	}
}

func (c *Config) normalize() {
	c.Heartbeat = parseHeartbeat(c.HeartbeatRaw)
	if c.Port == "" {
		c.Port = "8090"
	}
	if c.SQLitePath == "" {
		c.SQLitePath = "./data/agent.db"
	}
	if c.Workspace == "" {
		c.Workspace = "./workspace"
	}
	if c.AgentName == "" {
		c.AgentName = "worker-01"
	}
}

func parseHeartbeat(node yaml.Node) time.Duration {
	if node.Kind == 0 && node.Value == "" {
		return 15 * time.Second
	}
	var seconds int64
	if err := node.Decode(&seconds); err == nil {
		if seconds <= 0 {
			seconds = 15
		}
		return time.Duration(seconds) * time.Second
	}
	var raw string
	if err := node.Decode(&raw); err != nil || raw == "" {
		return 15 * time.Second
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 15 * time.Second
	}
	return d
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

const defaultConfigYAML = `AgentName: Saturn
AgentDescription: Просто мощный сервер
AgentPublicURL: http://127.0.0.1:8090/
CentralURL: http://127.0.0.1:8080/
Token: token
RabbitURL: amqp://dockrunner:dockrunner@localhost:5672/
RabbitTLSSkipVerify: false
SQLitePath: ./data/agent.db
Workspace: ./workspace
HostWorkspace: ""
Port: "8090"
Heartbeat: 15
# Victoria Metrics/Logs. Пусто — не отправлять.
# VictoriaMetricsURL: http://victoria:8428
# VictoriaLogsURL: http://victoria-logs:9428
# VictoriaToken: shared-bearer-token
# AgentIP: "192.168.1.100"  # Укажите IP агента вручную, если нужно переопределить автоопределение
`
