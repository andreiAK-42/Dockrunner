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
	"github.com/docker/docker/api/types/filters"
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
	// DeploymentID/AppName/Owner попадают в docker-метки: по ним агент
	// находит свой контейнер, если локальная SQLite потерялась.
	DeploymentID string
	AppName      string
	Owner        string
}

const (
	// LabelDeployment/LabelApp/LabelOwner — метки, по которым
	// восстанавливается потерянная запись о деплое (SQLite агента
	// пересоздали без volume). Owner нужен, чтобы не перепутать
	// одноимённые приложения разных пользователей.
	LabelDeployment = "dockrunner.deployment"
	LabelApp        = "dockrunner.app"
	LabelOwner      = "dockrunner.owner"
)

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

// buildArgsFromEnv превращает переменные окружения деплоя в build-args.
// Ключи, которые нельзя объявить в Dockerfile через ARG, пропускаем —
// сборка всё равно не смогла бы их использовать, а Docker может упасть
// на недопустимом имени.
func buildArgsFromEnv(env map[string]string) map[string]*string {
	if len(env) == 0 {
		return nil
	}
	args := make(map[string]*string, len(env))
	for k, v := range env {
		if !validBuildArgName(k) {
			continue
		}
		value := v
		args[k] = &value
	}
	if len(args) == 0 {
		return nil
	}
	return args
}

func validBuildArgName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func (c *Client) Build(ctx context.Context, contextDir, dockerfile, tag string, buildArgs map[string]string, onLine func(line string, progress bool)) error {
	tar := tarDirectory(contextDir)
	resp, err := c.cli.ImageBuild(ctx, tar, build.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{tag},
		BuildArgs:  buildArgsFromEnv(buildArgs),
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
	labels := map[string]string{}
	if spec.DeploymentID != "" {
		labels[LabelDeployment] = spec.DeploymentID
	}
	if spec.AppName != "" {
		labels[LabelApp] = spec.AppName
	}
	if spec.Owner != "" {
		labels[LabelOwner] = spec.Owner
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
		&container.Config{Image: spec.Image, Env: env, ExposedPorts: exposed, Entrypoint: spec.Entrypoint, Cmd: spec.Cmd, Labels: labels},
		hostConfig,
		netConfig,
		&ocispec.Platform{},
		spec.Name,
	)
	if err != nil && len(storageOpt) > 0 && strings.Contains(err.Error(), "storage-opt is supported only") {
		// Демон на ext4 или xfs без pquota — квота размера недоступна.
		// Запускаем без лимита диска, чтобы не ронять весь деплой.
		hostConfig.StorageOpt = map[string]string{}
		resp, err = c.cli.ContainerCreate(ctx,
			&container.Config{Image: spec.Image, Env: env, ExposedPorts: exposed, Entrypoint: spec.Entrypoint, Cmd: spec.Cmd, Labels: labels},
			hostConfig,
			netConfig,
			&ocispec.Platform{},
			spec.Name,
		)
	}
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

// Rename переименовывает контейнер (например, чтобы сохранить старую версию
// как резерв до успешного запуска новой).
func (c *Client) Rename(ctx context.Context, id, name string) error {
	return c.cli.ContainerRename(ctx, id, name)
}

// Start запускает ранее остановленный контейнер.
func (c *Client) Start(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

// FindIDByName ищет контейнер по точному имени, включая остановленные.
// Пустая строка означает, что контейнера с таким именем нет.
func (c *Client) FindIDByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	if name == "" {
		return "", nil
	}
	info, err := c.cli.ContainerInspect(ctx, name)
	if err != nil {
		if IsMissing(err) {
			return "", nil
		}
		return "", err
	}
	return info.ID, nil
}

func (c *Client) VolumeRemove(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, false)
}

// FindIDBySuffix находит контейнер (включая остановленные), имя которого
// заканчивается на suffix. Используется для самовосстановления записей
// о деплоях, потерянных из SQLite (например, после пересоздания контейнера
// агента без volume): имя контейнера держит <app>-<shortID деплоя>.
// Если пользователь задал своё имя контейнера без суффикса, дополнительно
// ищем по тегу образа: агент тегирует собранные образы как app<suffix>:latest.
func (c *Client) FindIDBySuffix(ctx context.Context, suffix string) (string, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	imagePrefix := "app" + suffix + ":"
	stoppedID := ""
	for _, item := range list {
		matched := false
		for _, name := range item.Names {
			if strings.HasSuffix(strings.TrimPrefix(name, "/"), suffix) {
				matched = true
				break
			}
		}
		if !matched && strings.HasPrefix(item.Image, imagePrefix) {
			matched = true
		}
		if !matched {
			continue
		}
		// Приоритет работающему контейнеру: остановленный мог остаться
		// от предыдущей попытки с тем же тегом образа.
		if item.State == "running" {
			return item.ID, nil
		}
		if stoppedID == "" {
			stoppedID = item.ID
		}
	}
	return stoppedID, nil
}

// FindIDByLabels находит контейнер (включая остановленные) по docker-меткам.
// Это основной способ восстановить деплой после потери SQLite: метки
// выставляются при создании контейнера и не зависят от его имени.
func (c *Client) FindIDByLabels(ctx context.Context, labels map[string]string) (string, error) {
	args := make([]filters.KeyValuePair, 0, len(labels))
	for key, value := range labels {
		args = append(args, filters.Arg("label", key+"="+value))
	}
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(args...)})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", nil
	}
	return list[0].ID, nil
}

// FindIDByApp находит контейнер приложения, даже если id деплоя уже другой:
// повторный деплой мог не дойти до создания контейнера, а старый контейнер
// приложения продолжает работать. Ищем по метке dockrunner.app, затем по
// точному имени и по имени "<app>-...", отдавая предпочтение запущенным.
// owner (если известен) отсекает чужие контейнеры с тем же именем приложения.
func (c *Client) FindIDByApp(ctx context.Context, appName, owner string) (string, error) {
	appName = strings.TrimSpace(strings.TrimPrefix(appName, "/"))
	if appName == "" {
		return "", nil
	}
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	bestScore := 0
	bestCreated := int64(0)
	bestID := ""
	for _, item := range list {
		if owner != "" {
			if labelOwner := item.Labels[LabelOwner]; labelOwner != "" && labelOwner != owner {
				continue
			}
		}
		score := 0
		if item.Labels[LabelApp] == appName {
			score = 3
		} else {
			for _, raw := range item.Names {
				name := strings.TrimPrefix(raw, "/")
				if name == appName {
					score = 2
					break
				}
				if strings.HasPrefix(name, appName+"-") {
					score = 1
				}
			}
		}
		if score == 0 {
			continue
		}
		if owner != "" && item.Labels[LabelOwner] == owner {
			score += 5
		}
		if item.State == "running" {
			score += 10
		}
		if score > bestScore || (score == bestScore && item.Created > bestCreated) {
			bestScore, bestCreated, bestID = score, item.Created, item.ID
		}
	}
	return bestID, nil
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
	// Контейнеры с TTY (например, созданные вручную в Portainer) отдают
	// не мультиплексированный поток: stdcopy на нём падает и лог получается
	// пустым, хотя docker logs его показывает.
	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return "", err
	}
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: false, Tail: tail,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	var out, errOut lineBuffer
	if tty {
		_, _ = io.Copy(&out, rc)
	} else {
		_, _ = stdcopy.StdCopy(&out, &errOut, rc)
	}
	out.Flush()
	errOut.Flush()
	return out.String() + errOut.String(), nil
}

// containerTTY сообщает, запущен ли контейнер с TTY.
func (c *Client) containerTTY(ctx context.Context, id string) (bool, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return false, err
	}
	return info.Config != nil && info.Config.Tty, nil
}

func (c *Client) FollowLogs(ctx context.Context, id string, emit func(stream, line string)) error {
	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return err
	}
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
	if tty {
		_, _ = io.Copy(out, rc)
	} else {
		_, _ = stdcopy.StdCopy(out, errOut, rc)
	}
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
