package git

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvVar represents an environment variable from .env file
type EnvVar struct {
	Key   string
	Value string
}

// ParseEnvFile parses a .env or .env.example file and returns a map of env vars
func ParseEnvFile(dir, filename string) (map[string]string, error) {
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	return parseEnvContent(string(data)), nil
}

// parseEnvContent parses the content of a .env file
func parseEnvContent(content string) map[string]string {
	env := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Handle export KEY=VALUE or KEY=VALUE
		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Remove quotes if present
			value = strings.Trim(value, "\"'`")
			if key != "" {
				env[key] = value
			}
		}
	}
	return env
}

// DetectEnvFiles finds .env and .env.example files and returns their parsed content
// Priority: .env.example first (as template), then .env (actual values override)
func DetectEnvFiles(dir string) (map[string]string, error) {
	envVars := make(map[string]string)

	// First load .env.example as template
	exampleEnv, err := ParseEnvFile(dir, ".env.example")
	if err != nil {
		return nil, err
	}
	for k, v := range exampleEnv {
		envVars[k] = v
	}

	// Then load .env which overrides
	actualEnv, err := ParseEnvFile(dir, ".env")
	if err != nil {
		return nil, err
	}
	for k, v := range actualEnv {
		envVars[k] = v
	}

	return envVars, nil
}
