package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type PortMapping struct {
	Host      int
	Container int
}

type VolumeMount struct {
	Name string
	Path string
}

type RunSpec struct {
	Name          string
	Image         string
	Env           map[string]string
	Ports         []PortMapping
	Volumes       []VolumeMount
	RestartPolicy string
	Entrypoint    []string
	Cmd           []string
	// CPUCores — лимит CPU на контейнер (0 = без лимита).
	CPUCores float64
	// MemoryGB — лимит RAM на контейнер в ГБ (0 = без лимита).
	MemoryGB float64
	// DiskGB — лимит диска на контейнер в ГБ (0 = без лимита).
	// Применяется через --storage-opt size=X для overlay2 (ограничивает writable layer).
	DiskGB float64
	// Network — пользовательская docker-сеть ("" = default).
	Network string
}

type Stats struct {
	CPUPercent float64
	MemMB      float64
}

type Client struct {
	cli *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) NetworkList(ctx context.Context, opts network.ListOptions) ([]network.Summary, error) {
	return c.cli.NetworkList(ctx, opts)
}

func IsMissing(err error) bool {
	return err != nil && errdefs.IsNotFound(err)
}

func (c *Client) EnsureImage(ctx context.Context, ref string) error {
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, rc)
	return rc.Close()
}

type buildMessage struct {
	Stream      string `json:"stream"`
	Status      string `json:"status"`
	Progress    string `json:"progress"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func buildErrorText(msg buildMessage) string {
	if msg.ErrorDetail.Message != "" {
		return msg.ErrorDetail.Message
	}
	return msg.Error
}

func statusText(msg buildMessage) string {
	if msg.Progress != "" {
		return msg.Status + " " + msg.Progress
	}
	return msg.Status
}

func (c *Client) Build(ctx context.Context, contextDir, dockerfile, tag string, onLine func(line string, progress bool)) error {
	tar := tarDirectory(contextDir)
	resp, err := c.cli.ImageBuild(ctx, tar, build.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{tag},
		Remove:     true,
	})
	_ = tar.Close()
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	emit := func(line string, progress bool) {
		if onLine != nil && line != "" {
			onLine(line, progress)
		}
	}
	var buildErr string
	dec := json.NewDecoder(resp.Body)
	for {
		var msg buildMessage
		if err := dec.Decode(&msg); err != nil {
			break
		}
		switch {
		case msg.Error != "":
			buildErr = buildErrorText(msg)
			emit("ERROR: "+buildErr, false)
		case msg.Stream != "":
			emit(strings.TrimRight(msg.Stream, "\n"), false)
		case msg.Status != "":
			emit(statusText(msg), true)
		}
	}
	if buildErr != "" {
		return errors.New(buildErr)
	}
	return nil
}

func (c *Client) Run(ctx context.Context, spec RunSpec) (string, error) {
	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, p := range spec.Ports {
		port, err := nat.NewPort("tcp", fmt.Sprint(p.Container))
		if err != nil {
			return "", err
		}
		exposed[port] = struct{}{}
		bindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprint(p.Host)}}
	}
	binds := make([]string, 0, len(spec.Volumes))
	for _, v := range spec.Volumes {
		binds = append(binds, v.Name+":"+v.Path)
	}
	policy := spec.RestartPolicy
	if policy == "" {
		policy = "unless-stopped"
	}
	resources := container.Resources{}
	if spec.CPUCores > 0 {
		resources.NanoCPUs = int64(spec.CPUCores * 1e9)
	}
	if spec.MemoryGB > 0 {
		resources.Memory = int64(spec.MemoryGB * 1024 * 1024 * 1024)
	}
	// StorageOpt для ограничения размера writable layer контейнера (overlay2.size)
	storageOpt := map[string]string{}
	if spec.DiskGB > 0 {
		storageOpt["size"] = fmt.Sprintf("%.0fG", spec.DiskGB)
	}
	netConfig := &network.NetworkingConfig{}
	hostConfig := &container.HostConfig{
		PortBindings:  bindings,
		Binds:         binds,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyMode(policy)},
		Resources:     resources,
		StorageOpt:    storageOpt,
	}
	if spec.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(spec.Network)
		netConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			spec.Network: {},
		}
	}
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{Image: spec.Image, Env: env, ExposedPorts: exposed, Entrypoint: spec.Entrypoint, Cmd: spec.Cmd},
		hostConfig,
		netConfig,
		&ocispec.Platform{},
		spec.Name,
	)
	if err != nil {
		return "", err
	}
	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return resp.ID, nil
}

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
		Env: map[string]string{}, Entrypoint: info.Config.Entrypoint, Cmd: info.Config.Cmd, WorkingDir: info.Config.WorkingDir, User: info.Config.User}
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
	if info.NetworkSettings != nil {
		for port, bindings := range info.NetworkSettings.Ports {
			containerPort, _ := strconv.Atoi(port.Port())
			for _, binding := range bindings {
				hostPort, _ := strconv.Atoi(binding.HostPort)
				if hostPort > 0 && containerPort > 0 {
					out.Ports = append(out.Ports, PortMapping{Host: hostPort, Container: containerPort})
				}
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
	if err := c.cli.ContainerRename(ctx, id, oldName+"-dockrunner-backup"); err != nil {
		return "", fmt.Errorf("rename old container: %w", err)
	}
	cfg := *info.Config
	// Keep original Entrypoint, Cmd, User from the container - not user-controllable for security
	cfg.Image = spec.Image
	cfg.Env = mapToEnv(spec.Env)
	cfg.WorkingDir = spec.WorkingDir
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
func mapToEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// EnsureNetwork создаёт пользовательскую сеть, если её ещё нет.
// Все сервисы одного compose-проекта садятся в неё и видят друг друга по именам.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	if _, err := c.cli.NetworkInspect(ctx, name, network.InspectOptions{}); err == nil {
		return nil
	}
	_, err := c.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"})
	return err
}

func (c *Client) Stop(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) Restart(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) Remove(ctx context.Context, id string) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: false})
}

func (c *Client) VolumeRemove(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, false)
}

func (c *Client) ContainerStatus(ctx context.Context, id string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	return info.State.Status, nil
}

func (c *Client) RunningIDs(ctx context.Context) ([]string, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		if item.State == "running" {
			ids = append(ids, item.ID)
		}
	}
	return ids, nil
}

func (c *Client) ContainerSizes(ctx context.Context) (map[string]int64, error) {
	usage, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{Types: []types.DiskUsageObject{types.ContainerObject}})
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(usage.Containers))
	for _, ctr := range usage.Containers {
		out[ctr.ID] = ctr.SizeRw
	}
	return out, nil
}

func readOneStat(ctx context.Context, cli *client.Client, id string) (container.StatsResponse, error) {
	res, err := cli.ContainerStats(ctx, id, false)
	if err != nil {
		return container.StatsResponse{}, err
	}
	defer func() { _ = res.Body.Close() }()
	var st container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		return container.StatsResponse{}, err
	}
	return st, nil
}

func (c *Client) Stats(ctx context.Context, id string) (Stats, error) {
	first, err := readOneStat(ctx, c.cli, id)
	if err != nil {
		return Stats{}, err
	}
	select {
	case <-ctx.Done():
		return Stats{}, ctx.Err()
	case <-time.After(time.Second):
	}
	second, err := readOneStat(ctx, c.cli, id)
	if err != nil {
		return Stats{}, err
	}
	cpuDelta := float64(second.CPUStats.CPUUsage.TotalUsage - first.CPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(second.CPUStats.SystemUsage - first.CPUStats.SystemUsage)
	cpu := 0.0
	if sysDelta > 0 {
		cpu = cpuDelta / sysDelta * float64(cpuCount(second)) * 100
	}
	return Stats{CPUPercent: cpu, MemMB: float64(second.MemoryStats.Usage) / 1024 / 1024}, nil
}

// cpuCount — число CPU по статистике: сначала OnlineCPUs,
// потом длина PercpuUsage, иначе 1 (деление на ноль исключено).
func cpuCount(st container.StatsResponse) int {
	if st.CPUStats.OnlineCPUs != 0 {
		return int(st.CPUStats.OnlineCPUs)
	}
	if len(st.CPUStats.CPUUsage.PercpuUsage) != 0 {
		return len(st.CPUStats.CPUUsage.PercpuUsage)
	}
	return 1
}

func (c *Client) LogsTail(ctx context.Context, id, tail string) (string, error) {
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: false, Tail: tail,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	var out, errOut lineBuffer
	_, _ = stdcopy.StdCopy(&out, &errOut, rc)
	out.Flush()
	errOut.Flush()
	return out.String() + errOut.String(), nil
}

func (c *Client) FollowLogs(ctx context.Context, id string, emit func(stream, line string)) error {
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true, Tail: "50",
	})
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	go func() {
		<-ctx.Done()
		_ = rc.Close()
	}()
	out := &lineWriter{emit: func(line string) { emit("stdout", line) }}
	errOut := &lineWriter{emit: func(line string) { emit("stderr", line) }}
	_, _ = stdcopy.StdCopy(out, errOut, rc)
	out.Flush()
	errOut.Flush()
	return nil
}

// splitLines достаёт из буфера законченные строки и отдаёт их в emit.
func splitLines(buf *strings.Builder, emit func(line string)) {
	for {
		s := buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		emit(s[:idx])
		buf.Reset()
		buf.WriteString(s[idx+1:])
	}
}

type lineBuffer struct {
	lines []string
	buf   strings.Builder
}

func (w *lineBuffer) Write(p []byte) (int, error) {
	w.buf.Write(p)
	splitLines(&w.buf, func(line string) {
		w.lines = append(w.lines, line)
	})
	return len(p), nil
}

func (w *lineBuffer) Flush() {
	if rest := w.buf.String(); rest != "" {
		w.lines = append(w.lines, rest)
		w.buf.Reset()
	}
}

func (w *lineBuffer) String() string {
	return strings.Join(w.lines, "\n")
}

type lineWriter struct {
	buf  strings.Builder
	emit func(line string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	splitLines(&w.buf, w.emit)
	return len(p), nil
}

func (w *lineWriter) Flush() {
	if rest := strings.TrimRight(w.buf.String(), "\r\n"); rest != "" {
		w.emit(rest)
	}
	w.buf.Reset()
}
