package git

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

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
