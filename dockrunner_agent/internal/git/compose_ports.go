package git

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

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
