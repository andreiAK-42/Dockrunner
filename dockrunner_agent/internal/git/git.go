package git

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func run(ctx context.Context, dir, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

func EnsureCloned(ctx context.Context, repoURL, dir, branch string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git not installed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return syncExisting(ctx, repoURL, dir, branch)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, dir)
	return run(ctx, "", "git", args...)
}

func syncExisting(ctx context.Context, repoURL, dir, branch string) error {
	if err := run(ctx, dir, "git", "remote", "set-url", "origin", repoURL); err != nil {
		return err
	}
	ref := "HEAD"
	if branch != "" {
		ref = branch
	}
	if err := run(ctx, dir, "git", "fetch", "--depth", "1", "origin", ref); err != nil {
		return err
	}
	return run(ctx, dir, "git", "reset", "--hard", "FETCH_HEAD")
}

func ListRoot(dir string) []string {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if item.IsDir() {
			name += "/"
		}
		names = append(names, name)
		if len(names) >= 20 {
			break
		}
	}
	return names
}

func Detect(dir string) (dockerfile string, compose string) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		lower := strings.ToLower(item.Name())
		if lower == "dockerfile" && dockerfile == "" {
			dockerfile = item.Name()
			continue
		}
		if compose == "" && isComposeFile(lower) {
			compose = item.Name()
		}
	}
	return dockerfile, compose
}

func isComposeFile(lower string) bool {
	switch lower {
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return true
	}
	return false
}

// ListRemoteBranches lists all branches from a remote repository without cloning
func ListRemoteBranches(ctx context.Context, repoURL string) ([]string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("git not installed")
	}
	// Use git ls-remote to get branches without cloning
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", repoURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	var branches []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <commit-hash>\trefs/heads/<branch-name>
		parts := strings.Split(line, "\t")
		if len(parts) == 2 && strings.HasPrefix(parts[1], "refs/heads/") {
			branchName := strings.TrimPrefix(parts[1], "refs/heads/")
			branches = append(branches, branchName)
		}
	}
	return branches, nil
}

// ListLocalBranches lists local branches in a repository.
// Remote-tracking refs (origin/main, origin/HEAD и т.п.) не возвращаем:
// это не ветки, и git fetch/clone --branch с ними падает
// ("couldn't find remote ref origin/main").
func ListLocalBranches(dir string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("git not installed")
	}
	cmd := exec.CommandContext(ctx, "git", "branch", "--format=%(refname:short)")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	var branches []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !seen[line] {
			branches = append(branches, line)
			seen[line] = true
		}
	}
	return branches, nil
}

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

// ExposedPort represents a port exposed by the container
type ExposedPort struct {
	ContainerPort int
	Protocol      string // "tcp" or "udp"
}

// ParseDockerfileExposedPorts parses a Dockerfile for EXPOSE instructions
func ParseDockerfileExposedPorts(dir, dockerfile string) ([]ExposedPort, error) {
	path := filepath.Join(dir, dockerfile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseDockerfileExposedPorts(string(data)), nil
}

var (
	exposeRegex = regexp.MustCompile(`(?i)^EXPOSE\s+(.+)$`)
	argEnvRegex = regexp.MustCompile(`(?i)^(ARG|ENV)\s+([A-Za-z_][A-Za-z0-9_]*)(?:[=\s]+(.*))?$`)
	varRefRegex = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)
)

func parseDockerfileExposedPorts(content string) []ExposedPort {
	vars := map[string]string{}
	var ports []ExposedPort
	for _, line := range dockerfileInstructions(content) {
		if matches := argEnvRegex.FindStringSubmatch(line); len(matches) >= 3 {
			key := matches[2]
			value := strings.TrimSpace(matches[3])
			value = strings.Trim(value, `"'`)
			if key != "" && value != "" {
				vars[key] = expandDockerfileVars(value, vars)
			}
			continue
		}
		matches := exposeRegex.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		spec := expandDockerfileVars(strings.TrimSpace(matches[1]), vars)
		for _, token := range exposeTokens(spec) {
			port, protocol := parsePortSpec(token)
			if port > 0 {
				ports = append(ports, ExposedPort{ContainerPort: port, Protocol: protocol})
			}
		}
	}
	return ports
}

func dockerfileInstructions(content string) []string {
	content = strings.TrimPrefix(content, "\ufeff")
	raw := strings.Split(content, "\n")
	var out []string
	var buf string
	for _, rawLine := range raw {
		line := stripDockerfileComment(strings.TrimRight(rawLine, "\r"))
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, "\\") {
			buf += strings.TrimSpace(strings.TrimSuffix(trimmed, "\\")) + " "
			continue
		}
		out = append(out, strings.TrimSpace(buf+trimmed))
		buf = ""
	}
	if strings.TrimSpace(buf) != "" {
		out = append(out, strings.TrimSpace(buf))
	}
	return out
}

func stripDockerfileComment(line string) string {
	var b strings.Builder
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '#' && !inSingle && !inDouble {
			break
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
		}
		b.WriteByte(ch)
	}
	return b.String()
}

func expandDockerfileVars(s string, vars map[string]string) string {
	return varRefRegex.ReplaceAllStringFunc(s, func(match string) string {
		groups := varRefRegex.FindStringSubmatch(match)
		key := groups[1]
		if key == "" {
			key = groups[2]
		}
		if val, ok := vars[key]; ok {
			return val
		}
		return match
	})
}

func exposeTokens(spec string) []string {
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "[") {
		var items []any
		if err := json.Unmarshal([]byte(spec), &items); err == nil {
			tokens := make([]string, 0, len(items))
			for _, item := range items {
				switch v := item.(type) {
				case string:
					tokens = append(tokens, v)
				case float64:
					tokens = append(tokens, strconv.Itoa(int(v)))
				}
			}
			return tokens
		}
	}
	return strings.Fields(spec)
}

func parsePortSpec(spec string) (int, string) {
	spec = strings.Trim(strings.TrimSpace(spec), `"'`)
	if spec == "" {
		return 0, ""
	}
	protocol := "tcp"
	portPart := spec
	if i := strings.Index(spec, "/"); i >= 0 {
		portPart = spec[:i]
		protocol = strings.ToLower(strings.TrimSpace(spec[i+1:]))
		if protocol == "" {
			protocol = "tcp"
		}
	}
	if dash := strings.Index(portPart, "-"); dash > 0 {
		portPart = portPart[:dash]
	}
	port := parsePortNumber(portPart)
	if port > 0 {
		return port, protocol
	}
	return 0, ""
}

func parsePortNumber(s string) int {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return 0
	}
	return n
}

func anyToPort(v any) int {
	switch n := v.(type) {
	case int:
		return parsePortNumber(strconv.Itoa(n))
	case int64:
		return parsePortNumber(strconv.FormatInt(n, 10))
	case uint64:
		return parsePortNumber(strconv.FormatUint(n, 10))
	case float64:
		if n == float64(int(n)) {
			return parsePortNumber(strconv.Itoa(int(n)))
		}
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return parsePortNumber(strconv.FormatInt(i, 10))
		}
	case string:
		return parsePortNumber(n)
	}
	return 0
}

// ParseComposePorts parses docker-compose file for ports and expose sections
func ParseComposePorts(dir, composeFile string) ([]ComposePort, error) {
	path := filepath.Join(dir, composeFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseComposePorts(string(data))
}

type composePortRaw struct {
	Ports  []any `yaml:"ports"`
	Expose []any `yaml:"expose"`
}

func parseComposePorts(content string) ([]ComposePort, error) {
	var parsed composeFileRaw
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, err
	}

	var ports []ComposePort
	for _, svc := range parsed.Services {
		// Parse ports
		for _, p := range svc.Ports {
			if port := parseComposePortMapping(p, false); port != nil {
				ports = append(ports, *port)
			}
		}
		// Parse expose
		for _, e := range svc.Expose {
			if port := parseComposePortMapping(e, true); port != nil {
				ports = append(ports, *port)
			}
		}
	}
	return ports, nil
}

func parseComposePortMapping(val any, isExpose bool) *ComposePort {
	switch v := val.(type) {
	case string:
		return parseComposePortString(v, isExpose)
	case int, int64, uint64, float64, json.Number:
		n := anyToPort(v)
		if n == 0 {
			return nil
		}
		return parseComposePortString(strconv.Itoa(n), isExpose)
	case map[string]any:
		return parseComposePortMap(v, isExpose)
	case map[any]any:
		converted := make(map[string]any, len(v))
		for key, value := range v {
			converted[fmt.Sprint(key)] = value
		}
		return parseComposePortMap(converted, isExpose)
	default:
		return nil
	}
}

func parseComposePortMap(v map[string]any, isExpose bool) *ComposePort {
	if target, ok := v["target"]; ok {
		containerPort := anyToPort(target)
		hostPort := anyToPort(v["published"])
		protocol := "tcp"
		if p, ok := v["protocol"].(string); ok && p != "" {
			protocol = strings.ToLower(p)
		}
		if isExpose {
			hostPort = 0
		}
		if containerPort > 0 {
			return &ComposePort{ContainerPort: containerPort, HostPort: hostPort, Protocol: protocol}
		}
		return nil
	}
	if len(v) != 1 {
		return nil
	}
	for key, value := range v {
		hostPort := parsePortNumber(key)
		containerPort := anyToPort(value)
		if containerPort == 0 && hostPort > 0 {
			containerPort = hostPort
			hostPort = 0
		}
		if isExpose {
			hostPort = 0
		}
		if containerPort > 0 {
			return &ComposePort{ContainerPort: containerPort, HostPort: hostPort, Protocol: "tcp"}
		}
	}
	return nil
}

func parseComposePortString(portStr string, isExpose bool) *ComposePort {
	portStr = strings.TrimSpace(strings.Trim(portStr, `"'`))
	if portStr == "" {
		return nil
	}

	protocol := "tcp"
	if i := strings.LastIndex(portStr, "/"); i >= 0 && !strings.Contains(portStr[i+1:], ":") {
		proto := strings.ToLower(strings.TrimSpace(portStr[i+1:]))
		if proto == "tcp" || proto == "udp" || proto == "sctp" {
			protocol = proto
			portStr = portStr[:i]
		}
	}

	parts := strings.Split(portStr, ":")
	var hostPort, containerPort int

	switch len(parts) {
	case 1:
		containerPort = parsePortNumber(parts[0])
	case 2:
		hostPort = parsePortNumber(parts[0])
		containerPort = parsePortNumber(parts[1])
		if containerPort == 0 {
			containerPort = hostPort
			hostPort = 0
		}
	default:
		hostPort = parsePortNumber(parts[len(parts)-2])
		containerPort = parsePortNumber(parts[len(parts)-1])
	}

	if isExpose {
		hostPort = 0
	}
	if containerPort > 0 {
		return &ComposePort{ContainerPort: containerPort, HostPort: hostPort, Protocol: protocol}
	}
	return nil
}

// ComposePort represents a port mapping from docker-compose
type ComposePort struct {
	ContainerPort int
	HostPort      int // 0 if not specified (expose only)
	Protocol      string
}

// ComposeService represents a service in docker-compose
type ComposeService struct {
	Name       string
	Image      string
	Dockerfile string
	ContextDir string
	Ports      []ComposePort
}

type composeServiceRaw struct {
	Image  string `yaml:"image"`
	Build  any    `yaml:"build"`
	Ports  []any  `yaml:"ports"`
	Expose []any  `yaml:"expose"`
}

type composeFileRaw struct {
	Services map[string]composeServiceRaw `yaml:"services"`
}

func parseComposeServicePorts(ports, expose []any) []ComposePort {
	var result []ComposePort
	for _, p := range ports {
		if port := parseComposePortMapping(p, false); port != nil {
			result = append(result, *port)
		}
	}
	for _, e := range expose {
		if port := parseComposePortMapping(e, true); port != nil {
			result = append(result, *port)
		}
	}
	return result
}
