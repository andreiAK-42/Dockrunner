package api

import (
	"encoding/json"
	"net/http"

	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/store"
)

type SpecPort struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

// UnmarshalJSON accepts both port shapes used in the system: the API/spec
// shape {host, container} and the deploy shape {host_port, container_port}.
// Update requests and old records use the latter.
func (p *SpecPort) UnmarshalJSON(data []byte) error {
	var raw struct {
		Host          int `json:"host"`
		Container     int `json:"container"`
		HostPort      int `json:"host_port"`
		ContainerPort int `json:"container_port"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Host = raw.Host
	if p.Host == 0 {
		p.Host = raw.HostPort
	}
	p.Container = raw.Container
	if p.Container == 0 {
		p.Container = raw.ContainerPort
	}
	return nil
}

type SpecVolume struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}
type SpecResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Status        string            `json:"status"`
	Ports         []SpecPort        `json:"ports"`
	Volumes       []SpecVolume      `json:"volumes"`
	Env           map[string]string `json:"env"`
	BuildArgs     map[string]string `json:"build_args"`
	Entrypoint    []string          `json:"entrypoint"`
	Cmd           []string          `json:"cmd"`
	RestartPolicy string            `json:"restart_policy"`
	Network       string            `json:"network"`
	WorkingDir    string            `json:"working_dir"`
	User          string            `json:"user"`
}

// storedSpec собирает ответ из записи SQLite — используется, пока контейнер
// ещё не создан (идёт сборка): центр получает 200 со статусом "building",
// а не ошибку, из-за которой он помечал контейнер «stopped».
func storedSpec(dep *store.Deployment) SpecResponse {
	var ports []SpecPort
	_ = json.Unmarshal([]byte(dep.Ports), &ports)
	if ports == nil {
		ports = []SpecPort{}
	}
	type storedVolume struct {
		Name      string `json:"name"`
		HostPath  string `json:"host_path"`
		Container string `json:"container_path"`
		Path      string `json:"path"`
	}
	var rawVols []storedVolume
	_ = json.Unmarshal([]byte(dep.Volumes), &rawVols)
	vols := make([]SpecVolume, 0, len(rawVols))
	for _, v := range rawVols {
		containerPath := v.Container
		if containerPath == "" {
			containerPath = v.Path
		}
		vols = append(vols, SpecVolume{Name: v.Name, HostPath: v.HostPath, ContainerPath: containerPath})
	}
	env := map[string]string{}
	_ = json.Unmarshal([]byte(dep.Env), &env)
	return SpecResponse{
		ID: dep.ID, Name: dep.Name, Image: dep.Image, Status: dep.Status,
		Ports: ports, Volumes: vols, Env: env, BuildArgs: docker.BuildArgsFromEnv(env),
		Entrypoint: []string{}, Cmd: []string{},
	}
}

func (s *Server) handleContainerSpec(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	if dep.Container == "" {
		writeJSON(w, http.StatusOK, storedSpec(dep))
		return
	}
	live, err := s.dock.InspectSpec(r.Context(), dep.Container)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "docker inspect failed: " + err.Error()})
		return
	}
	ports := make([]SpecPort, 0, len(live.Ports))
	for _, p := range live.Ports {
		ports = append(ports, SpecPort{Host: p.Host, Container: p.Container})
	}
	vols := make([]SpecVolume, 0, len(live.Volumes))
	for _, v := range live.Volumes {
		vols = append(vols, SpecVolume{Name: v.Name, HostPath: v.HostPath, ContainerPath: v.ContainerPath, ReadOnly: v.ReadOnly})
	}
	writeJSON(w, http.StatusOK, SpecResponse{
		ID: live.ID, Name: live.Name, Image: live.Image, Status: live.Status,
		Ports: ports, Volumes: vols, Env: live.Env, BuildArgs: docker.BuildArgsFromEnv(live.Env),
		Entrypoint: live.Entrypoint, Cmd: live.Cmd, RestartPolicy: live.RestartPolicy,
		Network: live.Network, WorkingDir: live.WorkingDir, User: live.User,
	})
}
