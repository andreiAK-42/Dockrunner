package worker

import (
	"errors"
	"strings"

	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/fsx"
)

func allowedNameRune(r rune) bool {
	isLetter := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	isDigit := r >= '0' && r <= '9'
	isExtra := r == '_' || r == '.' || r == '-'
	return isLetter || isDigit || isExtra
}

func sanitizeContainerName(appName, custom, deploymentID string) (string, error) {
	name := custom
	if name == "" {
		name = appName + "-" + shortID(deploymentID)
	}
	clean := strings.Map(func(r rune) rune {
		if allowedNameRune(r) {
			return r
		}
		return '-'
	}, name)
	clean = strings.Trim(clean, "_.-")
	if clean == "" {
		return "", errors.New("bad container name")
	}
	return clean, nil
}

// shortID condenses a deployment id for human-readable image tags
// and default container names (first 8 chars are unique enough here).
func shortID(id string) string {
	id = strings.ToLower(id)
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func sanitizeVolumeName(name string) (string, error) {
	clean := strings.Map(func(r rune) rune {
		if allowedNameRune(r) {
			return r
		}
		return -1
	}, name)
	if clean == "" || clean != name {
		return "", errBadVolumeName
	}
	return clean, nil
}

func containerPathOf(v VolumeTask) string {
	if v.Container != "" {
		return v.Container
	}
	return v.Path
}

func (h *Handler) resolveVolumes(task Task) ([]docker.VolumeMount, error) {
	volumes := make([]docker.VolumeMount, 0, len(task.Volumes))
	for _, v := range task.Volumes {
		containerPath := containerPathOf(v)
		if containerPath == "" {
			return nil, errBadVolume
		}
		if v.HostPath != "" {
			abs, err := fsx.ResolveHostPath(h.workspace, task.Owner, v.HostPath)
			if err != nil {
				return nil, err
			}
			// Путь для демона: если агент в контейнере, хост видит тот же
			// каталог по другому адресу (см. HOST_WORKSPACE_DIR).
			volumes = append(volumes, docker.VolumeMount{Name: h.toDaemonPath(abs), Path: containerPath})
			continue
		}
		volumes = append(volumes, docker.VolumeMount{Name: v.Name, Path: containerPath})
	}
	return volumes, nil
}

func toPortMappings(ports []PortTask) ([]docker.PortMapping, error) {
	mapped := make([]docker.PortMapping, 0, len(ports))
	for _, p := range ports {
		hostOK := p.Host > 0 && p.Host <= 65535
		containerOK := p.Container > 0 && p.Container <= 65535
		if !hostOK || !containerOK {
			return nil, errBadPort
		}
		mapped = append(mapped, docker.PortMapping{Host: p.Host, Container: p.Container})
	}
	return mapped, nil
}
