package docker

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// LiveSpec возвращает фактическую конфигурацию из Docker inspect, а не сохранённую
// при деплое копию из SQLite. Это источник истины для команды запуска.
type LiveSpec struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Status        string            `json:"status"`
	Env           map[string]string `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []LiveVolume      `json:"volumes"`
	Entrypoint    []string          `json:"entrypoint"`
	Cmd           []string          `json:"cmd"`
	RestartPolicy string            `json:"restart_policy"`
	Network       string            `json:"network"`
	WorkingDir    string            `json:"working_dir"`
	User          string            `json:"user"`
	Labels        map[string]string `json:"labels,omitempty"`
}
type LiveVolume struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

func (c *Client) InspectSpec(ctx context.Context, id string) (LiveSpec, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return LiveSpec{}, err
	}
	out := LiveSpec{ID: info.ID, Name: strings.TrimPrefix(info.Name, "/"), Image: info.Config.Image, Status: info.State.Status,
		Env: map[string]string{}, Entrypoint: info.Config.Entrypoint, Cmd: info.Config.Cmd, WorkingDir: info.Config.WorkingDir, User: info.Config.User,
		Labels: info.Config.Labels}
	if info.HostConfig != nil {
		out.RestartPolicy = string(info.HostConfig.RestartPolicy.Name)
	}
	if info.NetworkSettings != nil {
		for name := range info.NetworkSettings.Networks {
			out.Network = name
			break
		}
	}
	for _, item := range info.Config.Env {
		k, v, ok := strings.Cut(item, "=")
		if ok {
			out.Env[k] = v
		}
	}
	out.Ports = []PortMapping{}
	seenPorts := map[[2]int]bool{}
	addPort := func(hostPort, containerPort int) {
		if hostPort <= 0 || containerPort <= 0 {
			return
		}
		key := [2]int{hostPort, containerPort}
		if seenPorts[key] {
			return
		}
		seenPorts[key] = true
		out.Ports = append(out.Ports, PortMapping{Host: hostPort, Container: containerPort})
	}
	if info.NetworkSettings != nil {
		for port, bindings := range info.NetworkSettings.Ports {
			containerPort, _ := strconv.Atoi(port.Port())
			for _, binding := range bindings {
				hostPort, _ := strconv.Atoi(binding.HostPort)
				addPort(hostPort, containerPort)
			}
		}
	}
	// У остановленного контейнера NetworkSettings.Ports может быть пустым,
	// но привязки портов остаются в HostConfig.PortBindings.
	if len(out.Ports) == 0 && info.HostConfig != nil {
		for port, bindings := range info.HostConfig.PortBindings {
			containerPort, _ := strconv.Atoi(port.Port())
			for _, binding := range bindings {
				hostPort, _ := strconv.Atoi(binding.HostPort)
				addPort(hostPort, containerPort)
			}
		}
	}
	out.Volumes = []LiveVolume{}
	for _, mount := range info.Mounts {
		out.Volumes = append(out.Volumes, LiveVolume{Name: mount.Name, HostPath: mount.Source, ContainerPath: mount.Destination, ReadOnly: !mount.RW})
	}
	return out, nil
}

// RecreateFromSpec пересоздаёт контейнер с изменёнными параметрами. Старый контейнер
// удаляется только после успешной остановки; его именованные/внешние тома не удаляются.
type RecreateSpec struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Env           map[string]string `json:"env"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMount     `json:"volumes"`
	RestartPolicy string            `json:"restart_policy"`
	Network       string            `json:"network"`
	WorkingDir    string            `json:"working_dir"`
	// DeploymentID/AppName/Owner обновляют docker-метки при пересоздании:
	// у контейнеров, созданных до появления меток, иначе их так и не будет.
	DeploymentID string `json:"-"`
	AppName      string `json:"-"`
	Owner        string `json:"-"`
}

func (c *Client) Recreate(ctx context.Context, id string, spec RecreateSpec) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(spec.Name) == "" {
		spec.Name = strings.TrimPrefix(info.Name, "/")
	}
	if strings.TrimSpace(spec.Image) == "" {
		spec.Image = info.Config.Image
	}
	if spec.Env == nil {
		spec.Env = map[string]string{}
	}
	if spec.RestartPolicy == "" && info.HostConfig != nil {
		spec.RestartPolicy = string(info.HostConfig.RestartPolicy.Name)
	}
	if spec.RestartPolicy == "" {
		spec.RestartPolicy = "unless-stopped"
	}
	if _, _, inspectErr := c.cli.ImageInspectWithRaw(ctx, spec.Image); inspectErr != nil {
		pull, pullErr := c.cli.ImagePull(ctx, spec.Image, image.PullOptions{})
		if pullErr != nil {
			return "", fmt.Errorf("pull image: %w", pullErr)
		}
		_, copyErr := io.Copy(io.Discard, pull)
		closeErr := pull.Close()
		if copyErr != nil {
			return "", fmt.Errorf("read image pull response: %w", copyErr)
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	if info.State != nil && info.State.Running {
		timeout := 10
		if err := c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout}); err != nil {
			return "", err
		}
	}
	// Keep old container as rollback until new config is validated by Docker create.
	oldName := strings.TrimPrefix(info.Name, "/")
	backupName := oldName + "-dockrunner-backup"
	// Остаток прошлой неудачной замены занял бы имя резерва.
	_ = c.cli.ContainerRemove(ctx, backupName, container.RemoveOptions{Force: true, RemoveVolumes: false})
	if err := c.cli.ContainerRename(ctx, id, backupName); err != nil {
		if info.State != nil && info.State.Running {
			_ = c.cli.ContainerStart(ctx, id, container.StartOptions{})
		}
		return "", fmt.Errorf("rename old container: %w", err)
	}
	cfg := *info.Config
	// Keep original Entrypoint, Cmd, User from the container - not user-controllable for security
	cfg.Image = spec.Image
	cfg.Env = mapToEnv(spec.Env)
	cfg.WorkingDir = spec.WorkingDir
	// Копируем метки, чтобы не менять info.Config, и обновляем служебные:
	// пересозданный контейнер должен находиться по метке деплоя/приложения.
	labels := make(map[string]string, len(cfg.Labels)+3)
	for k, v := range cfg.Labels {
		labels[k] = v
	}
	if spec.DeploymentID != "" {
		labels[LabelDeployment] = spec.DeploymentID
	}
	if spec.AppName != "" {
		labels[LabelApp] = spec.AppName
	}
	if spec.Owner != "" {
		labels[LabelOwner] = spec.Owner
	}
	cfg.Labels = labels
	host := *info.HostConfig
	host.RestartPolicy = container.RestartPolicy{Name: container.RestartPolicyMode(spec.RestartPolicy)}
	if spec.Network != "" {
		host.NetworkMode = container.NetworkMode(spec.Network)
	}
	host.PortBindings = nat.PortMap{}
	cfg.ExposedPorts = nat.PortSet{}
	for _, p := range spec.Ports {
		port, e := nat.NewPort("tcp", fmt.Sprint(p.Container))
		if e != nil {
			_ = c.cli.ContainerRename(ctx, id, oldName)
			return "", e
		}
		cfg.ExposedPorts[port] = struct{}{}
		host.PortBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprint(p.Host)}}
	}
	host.Binds = nil
	for _, v := range spec.Volumes {
		if v.Name == "" {
			continue
		}
		host.Binds = append(host.Binds, v.Name+":"+v.Path)
	}
	endpoints := map[string]*network.EndpointSettings{}
	if info.NetworkSettings != nil && info.NetworkSettings.Networks != nil {
		endpoints = info.NetworkSettings.Networks
	}
	if spec.Network != "" {
		switch spec.Network {
		case "bridge", "host", "none":
			endpoints = map[string]*network.EndpointSettings{}
		default:
			endpoints = map[string]*network.EndpointSettings{spec.Network: {}}
		}
	}
	created, err := c.cli.ContainerCreate(ctx, &cfg, &host, &network.NetworkingConfig{EndpointsConfig: endpoints}, nil, spec.Name)
	if err != nil {
		_ = c.cli.ContainerRename(ctx, id, oldName)
		return "", fmt.Errorf("create replacement: %w", err)
	}
	if err = c.cli.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		_ = c.cli.ContainerRemove(ctx, created.ID, container.RemoveOptions{Force: true, RemoveVolumes: false})
		_ = c.cli.ContainerRename(ctx, id, oldName)
		return "", fmt.Errorf("start replacement: %w", err)
	}
	// New container is healthy enough to start; remove the old one without deleting volumes.
	_ = c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: false})
	return created.ID, nil
}
