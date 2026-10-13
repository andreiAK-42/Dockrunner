package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	// CPUCores — лимит CPU на контейнер (0 = без лимита).
	CPUCores float64
	// MemoryGB — лимит RAM на контейнер в ГБ (0 = без лимита).
	MemoryGB float64
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
	netConfig := &network.NetworkingConfig{}
	hostConfig := &container.HostConfig{
		PortBindings:  bindings,
		Binds:         binds,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyMode(policy)},
		Resources:     resources,
	}
	if spec.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(spec.Network)
		netConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			spec.Network: {},
		}
	}
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{Image: spec.Image, Env: env, ExposedPorts: exposed},
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
