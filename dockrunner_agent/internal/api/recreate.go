package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/docker/docker/api/types/network"

	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/store"
)

type RecreateRequest struct {
	Name  string            `json:"name"`
	Image string            `json:"image"`
	Env   map[string]string `json:"env"`
	// Ports is a pointer to tell "not provided" (keep current mapping) from
	// an explicit empty list (a forbidden attempt to remove the mapping).
	Ports         *[]SpecPort  `json:"ports"`
	Volumes       []SpecVolume `json:"volumes"`
	RestartPolicy string       `json:"restart_policy"`
	Network       string       `json:"network"`
	WorkingDir    string       `json:"working_dir"`
}

// storedPort — форма, в которой порты лежат в SQLite агента (как в deploy).
type storedPort struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

func (s *Server) handleContainerRecreate(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	var req RecreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(req.Image) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image is required"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = dep.Name
	}
	// Fundamental parameters identify the running container. Changing them
	// through the launch editor would silently move the app to another port or
	// image, so only a full redeploy/update may do that.
	if dep.Name != "" && req.Name != dep.Name {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "имя запущенного контейнера изменить нельзя"})
		return
	}
	if dep.Image != "" && req.Image != dep.Image {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "образ запущенного контейнера изменить нельзя — используйте обновление"})
		return
	}
	if req.Ports != nil {
		if !samePorts(storedPorts(dep.Ports), *req.Ports) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "порты запущенного контейнера изменить нельзя"})
			return
		}
	}
	// Network is fundamental as well: a recreate may keep the current network,
	// but switching it would disconnect the app from its compose neighbours.
	currentNetwork := ""
	if live, err := s.dock.InspectSpec(r.Context(), dep.Container); err == nil {
		currentNetwork = live.Network
	}
	if strings.TrimSpace(req.Network) == "" {
		req.Network = currentNetwork
	}
	if req.Network != "" {
		if currentNetwork != "" {
			if req.Network != currentNetwork {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "сеть запущенного контейнера изменить нельзя"})
				return
			}
		} else {
			// Container could not be inspected: keep the old whitelist behaviour.
			allowed, err := s.allowedNetworks(r.Context())
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to list networks"})
				return
			}
			if !allowed[req.Network] {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "network not allowed"})
				return
			}
		}
	}
	// Validate working_dir
	if err := validateWorkingDir(req.WorkingDir); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	requestedPorts := storedPorts(dep.Ports)
	if req.Ports != nil {
		requestedPorts = *req.Ports
	}
	ports := make([]docker.PortMapping, 0, len(requestedPorts))
	for _, p := range requestedPorts {
		if p.Host < 1 || p.Host > 65535 || p.Container < 1 || p.Container > 65535 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid port mapping"})
			return
		}
		ports = append(ports, docker.PortMapping{Host: p.Host, Container: p.Container})
	}
	vols := make([]docker.VolumeMount, 0, len(req.Volumes))
	for _, v := range req.Volumes {
		path := v.HostPath
		if path == "" {
			path = v.Name
		}
		if path == "" || v.ContainerPath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "volume source and container path are required"})
			return
		}
		vols = append(vols, docker.VolumeMount{Name: path, Path: v.ContainerPath})
	}
	// Entrypoint, Cmd, User are intentionally ignored - use image defaults
	id, err := s.dock.Recreate(r.Context(), dep.Container, docker.RecreateSpec{
		Name:          req.Name,
		Image:         req.Image,
		Env:           req.Env,
		Ports:         ports,
		Volumes:       vols,
		RestartPolicy: req.RestartPolicy,
		Network:       req.Network,
		WorkingDir:    req.WorkingDir,
		DeploymentID:  dep.ID,
		AppName:       dep.AppName,
		Owner:         dep.Owner,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = store.UpdateDeploymentStatus(s.db, dep.ID, "running", id)
	dep.Container = id
	dep.Name = req.Name
	dep.Image = req.Image
	// Keep the same shape deploy/refresh use, otherwise the next refresh reads
	// zero ports and cannot rebuild the container correctly.
	portsToStore := make([]storedPort, 0, len(requestedPorts))
	for _, p := range requestedPorts {
		portsToStore = append(portsToStore, storedPort{HostPort: p.Host, ContainerPort: p.Container})
	}
	portsJSON, _ := json.Marshal(portsToStore)
	volumesJSON, _ := json.Marshal(req.Volumes)
	envJSON, _ := json.Marshal(req.Env)
	dep.Ports = string(portsJSON)
	dep.Volumes = string(volumesJSON)
	dep.Env = string(envJSON)
	dep.Status = "running"
	if err := store.SaveDeployment(s.db, *dep); err != nil {
		s.logger("recreate: record not saved", "deployment", dep.ID, "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "container_id": id, "status": "running"})
}

// storedPorts decodes a deployment's saved ports. SpecPort accepts both the
// API shape ({host, container}) and the deploy shape ({host_port,
// container_port}), so old records keep working.
func storedPorts(raw string) []SpecPort {
	if raw == "" {
		return nil
	}
	var ports []SpecPort
	if err := json.Unmarshal([]byte(raw), &ports); err != nil {
		return nil
	}
	valid := make([]SpecPort, 0, len(ports))
	for _, p := range ports {
		if p.Host > 0 && p.Container > 0 {
			valid = append(valid, p)
		}
	}
	return valid
}

// samePorts compares mappings as sets: order and the wording of an unchanged
// command must not matter.
func samePorts(a, b []SpecPort) bool {
	if len(a) != len(b) {
		return false
	}
	left := make(map[SpecPort]int, len(a))
	for _, p := range a {
		left[p]++
	}
	for _, p := range b {
		if left[p] == 0 {
			return false
		}
		left[p]--
	}
	return true
}

// allowedNetworks returns the list of networks the user is allowed to use.
func (s *Server) allowedNetworks(ctx context.Context) (map[string]bool, error) {
	list, err := s.dock.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{
		"bridge": true,
		"host":   true,
		"none":   true,
	}
	for _, n := range list {
		// Allow networks created by this agent (named like dockrunner-*)
		if strings.HasPrefix(n.Name, "dockrunner-") {
			allowed[n.Name] = true
		}
	}
	return allowed, nil
}

// validateWorkingDir проверяет рабочий каталог контейнера. Путь применяется
// только внутри контейнера, поэтому достаточно потребовать абсолютный путь:
// белый список ломал валидные значения из образов (например, "/" у nginx).
func validateWorkingDir(dir string) error {
	if dir == "" {
		return nil
	}
	clean := path.Clean(dir)
	if !path.IsAbs(clean) {
		return fmt.Errorf("working_dir must be an absolute path inside the container")
	}
	return nil
}
