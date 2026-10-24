package config

import (
	"fmt"
	"os"
	"time"

	"net/url"

	"gopkg.in/yaml.v3"
)

const Version = "0.1.0"

const configFile = "config.yaml"

// Config — YAML-конфиг агента. Поля City/Lat/Lng/IP в файл не пишутся:
// город и координаты определяются при старте, IP — из geo или локального интерфейса.
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
	return &cfg, nil
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
`
