package worker

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"dockrunner_agent/internal/buildlog"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/files"
	"dockrunner_agent/internal/fsx"
	"dockrunner_agent/internal/git"
	"dockrunner_agent/internal/queue"
	"dockrunner_agent/internal/store"
)

var errRebootUnsupported = errors.New("host reboot supported on linux only")
var errNoSource = errors.New("no image or repository to deploy")
var errComposeEmpty = errors.New("compose service has neither build nor image")
var errNoDockerfile = errors.New("Dockerfile or compose file not found in project root")
var errBadVolume = errors.New("volume needs a container path")
var errBadVolumeName = errors.New("bad volume name")
var errBadPort = errors.New("bad port mapping")

type PortTask struct {
	Host      int `json:"host_port"`
	Container int `json:"container_port"`
}

type VolumeTask struct {
	Name      string `json:"name"`
	HostPath  string `json:"host_path"`
	Container string `json:"container_path"`
	Path      string `json:"path"`
}

type Task struct {
	Type          string            `json:"type"`
	DeploymentID  string            `json:"deployment_id"`
	AppID         string            `json:"app_id"`
	AppName       string            `json:"app_name"`
	Owner         string            `json:"owner"`
	RepoURL       string            `json:"repo_url"`
	Image         string            `json:"image"`
	ContainerName string            `json:"container_name"`
	VolumeName    string            `json:"volume_name"`
	Path          string            `json:"path"`
	ContentB64    string            `json:"content_b64"`
	Ports         []PortTask        `json:"ports"`
	Volumes       []VolumeTask      `json:"volumes"`
	Env           map[string]string `json:"env"`
	Restart       string            `json:"restart_policy"`
	// CPUCores/MemoryGB/DiskGB — лимиты НА КАЖДЫЙ контейнер (0 = без лимита).
	CPUCores float64 `json:"cpu_cores"`
	MemoryGB float64 `json:"memory_gb"`
	DiskGB   float64 `json:"disk_gb"`
	// Services — сервисы compose для запуска (пусто = автоопределение из репозитория).
	Services []string `json:"services"`
}

type Event struct {
	Type               string   `json:"type"`
	AgentID            string   `json:"agent_id"`
	AppID              string   `json:"app_id"`
	AppName            string   `json:"app_name"`
	OwnerLogin         string   `json:"owner_login"`
	Image              string   `json:"image"`
	DeploymentID       string   `json:"deployment_id"`
	DetectedDockerfile bool     `json:"detected_dockerfile"`
	ComposeFile        string   `json:"compose_file"`
	Services           []string `json:"services,omitempty"`
	// ParentApp — головное приложение для событий сервис-контейнеров
	// (нужно центру, чтобы снять oplock родительской операции).
	ParentApp string     `json:"parent_app,omitempty"`
	Ports     []PortTask `json:"ports"`
	Message   string     `json:"msg"`
	Time      string     `json:"time"`
}

type Handler struct {
	agentID   string
	workspace string
	// hostWorkspace — тот же каталог глазами docker-демона (хоста).
	// Нужен только если агент в контейнере и пути различаются:
	// бинды для чужих контейнеров демон резолвит на хосте, а не в агенте.
	hostWorkspace string
	db            *sql.DB
	dock          *docker.Client
	queue         *queue.Client
	logger        *slog.Logger
	builds        *buildlog.Store
	onReady       func(containerID, containerName string)
	onStopped     func(containerID string)
	mu            sync.Mutex
	appLocks      map[string]*sync.Mutex
	hostMu        sync.RWMutex
}

func NewHandler(agentID, workspace string, db *sql.DB, dock *docker.Client, queue *queue.Client, logger *slog.Logger, builds *buildlog.Store, onReady func(string, string), onStopped func(string)) *Handler {
	if builds == nil {
		builds = buildlog.New(db)
	}
	return &Handler{
		agentID:   agentID,
		workspace: workspace,
		db:        db,
		dock:      dock,
		queue:     queue,
		logger:    logger,
		builds:    builds,
		onReady:   onReady,
		onStopped: onStopped,
		appLocks:  map[string]*sync.Mutex{},
	}
}

// SetHostWorkspace задаёт хостовый путь workspace для биндов.
// Пусто или равно workspace — трансляции нет (агент бинарником на хосте).
func (h *Handler) SetHostWorkspace(hostWorkspace string) {
	h.hostWorkspace = hostWorkspace
}

// toDaemonPath пересчитывает путь из файловой системы агента в путь,
// каким его увидит docker-демон (хост). Вне workspace — как есть.
func (h *Handler) toDaemonPath(abs string) string {
	if h.hostWorkspace == "" || h.hostWorkspace == h.workspace {
		return abs
	}
	rel, err := filepath.Rel(h.workspace, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return abs
	}
	return filepath.Join(h.hostWorkspace, rel)
}

// lockApp serializes tasks touching one project dir (owner/app).
// Reboot takes the write side — no app task runs during host reboot.
func (h *Handler) lockApp(task Task) func() {
	key := task.Owner + "/" + task.AppName
	h.mu.Lock()
	l, ok := h.appLocks[key]
	if !ok {
		l = &sync.Mutex{}
		h.appLocks[key] = l
	}
	h.mu.Unlock()
	h.hostMu.RLock()
	l.Lock()
	return func() {
		l.Unlock()
		h.hostMu.RUnlock()
	}
}

func (h *Handler) publishEvent(ctx context.Context, event Event) {
	if h.queue == nil {
		return
	}
	event.AgentID = h.agentID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	body, _ := json.Marshal(event)
	if err := h.queue.Publish(ctx, queue.QueueEvents, body); err != nil {
		h.logger.Warn("publish event failed", "error", err)
		return
	}
	h.logger.Info("event published", "type", event.Type, "deployment", event.DeploymentID)
}

func (h *Handler) Handle(ctx context.Context, msg amqp.Delivery) error {
	var task Task
	if err := json.Unmarshal(msg.Body, &task); err != nil {
		return err
	}
	h.logger.Info("task received", "type", task.Type, "deployment", task.DeploymentID)
	if task.Type == "reboot" {
		h.hostMu.Lock()
		defer h.hostMu.Unlock()
		return h.reboot(ctx)
	}
	unlock := h.lockApp(task)
	defer unlock()
	switch task.Type {
	case "prepare":
		return h.prepare(ctx, task)
	case "deploy":
		return h.deploy(ctx, task)
	case "stop":
		return h.stop(ctx, task)
	case "restart":
		return h.restart(ctx, task)
	case "remove":
		return h.remove(ctx, task)
	case "volume.remove":
		return h.volumeRemove(ctx, task)
	case "refresh":
		return h.refresh(ctx, task)
	case "files.list", "files.mkdir", "files.upload", "files.download", "files.delete":
		return h.replyFiles(ctx, msg, task)
	default:
		h.logger.Warn("unknown task type", "type", task.Type)
		return nil
	}
}

func failResponse(err error) map[string]any {
	return map[string]any{"ok": false, "error": err.Error()}
}

func badContentResponse() map[string]any {
	return map[string]any{"ok": false, "error": "bad content"}
}

func (h *Handler) replyFiles(ctx context.Context, msg amqp.Delivery, task Task) error {
	response := map[string]any{"ok": true}

	switch task.Type {
	case "files.list":
		entries, err := files.List(h.workspace, task.Owner, task.Path)
		if err != nil {
			response = failResponse(err)
			break
		}
		response["entries"] = entries
	case "files.mkdir":
		if err := files.Mkdir(h.workspace, task.Owner, task.Path); err != nil {
			response = failResponse(err)
		}
	case "files.upload":
		data, err := base64.StdEncoding.DecodeString(task.ContentB64)
		if err != nil {
			response = badContentResponse()
			break
		}
		if err := files.Write(h.workspace, task.Owner, task.Path, data); err != nil {
			response = failResponse(err)
		}
	case "files.download":
		data, err := files.Read(h.workspace, task.Owner, task.Path)
		if err != nil {
			response = failResponse(err)
			break
		}
		response["content_b64"] = base64.StdEncoding.EncodeToString(data)
	case "files.delete":
		if err := files.Delete(h.workspace, task.Owner, task.Path); err != nil {
			response = failResponse(err)
		}
	}

	body, _ := json.Marshal(response)
	return h.queue.Reply(ctx, msg, body)
}

func appEvent(eventType string, task Task, msg string) Event {
	return Event{
		Type:         eventType,
		AppID:        task.AppID,
		AppName:      task.AppName,
		OwnerLogin:   task.Owner,
		Image:        task.Image,
		DeploymentID: task.DeploymentID,
		Ports:        task.Ports,
		Message:      msg,
	}
}

// missingDeployment reports an operation on an unknown local deployment:
// no retry will fix it, so the task is dropped after notifying central.
func (h *Handler) missingDeployment(ctx context.Context, task Task, what string) error {
	h.logger.Warn("unknown local deployment", "type", task.Type, "deployment", task.DeploymentID)
	h.publishEvent(ctx, appEvent("op.failed", task, what+": развёртывание не найдено на узле — сначала запустите приложение"))
	return queue.Permanent(store.ErrNoDeployment)
}

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

// corruptedProjectDir — папка есть, а .git в ней нет:
// такой проект уже не синхронизировать, только удалять вручную.
func corruptedProjectDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return false
	}
	_, err := os.Stat(dir)
	return err == nil
}

func (h *Handler) projectDir(task Task) (string, error) {
	return fsx.ProjectDir(h.workspace, task.Owner, task.AppName)
}

func (h *Handler) prepare(ctx context.Context, task Task) error {
	fail := func(msg string) {
		h.publishEvent(ctx, Event{Type: "prepare.failed", AppID: task.AppID, AppName: task.AppName, DeploymentID: task.DeploymentID, Message: msg})
	}
	dir, err := h.projectDir(task)
	if err != nil {
		fail(err.Error())
		return nil
	}
	h.logger.Info("prepare started", "dir", dir, "repo", task.RepoURL)
	if corruptedProjectDir(dir) {
		fail("папка проекта повреждена (нет .git) — удалите её на воркере: " + dir)
		return nil
	}
	if err := git.EnsureCloned(ctx, task.RepoURL, dir); err != nil {
		fail("репозиторий не склонирован: " + err.Error())
		return err
	}
	dockerfile, compose := git.Detect(dir)
	names := git.ListRoot(dir)
	h.logger.Info("prepare detected", "dir", dir, "dockerfile", dockerfile, "compose", compose, "root", names)
	if dockerfile == "" && compose == "" {
		seen := strings.Join(names, ", ")
		if seen == "" {
			seen = "папка пуста"
		}
		fail("Dockerfile или docker-compose не найден в корне проекта (там: " + seen + ")")
		return nil
	}
	var services []string
	if compose != "" {
		// Имена сервисов нужны центру для цены (N контейнеров) и заявке.
		// Ошибка парсинга здесь не фатальна — упадёт на deploy с понятным текстом.
		services, _ = git.ParseComposeNames(dir, compose)
	}
	h.publishEvent(ctx, Event{
		Type:               "prepare.completed",
		AppID:              task.AppID,
		AppName:            task.AppName,
		DeploymentID:       task.DeploymentID,
		DetectedDockerfile: dockerfile != "",
		ComposeFile:        compose,
		Services:           services,
		Message:            "проект готов к сборке",
	})
	return nil
}

// pullImage тянет готовый образ. Пустая ссылка — ошибка: деплоить нечего.
func (h *Handler) pullImage(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", errNoSource
	}
	if err := h.dock.EnsureImage(ctx, ref); err != nil {
		return "", err
	}
	return ref, nil
}

// buildComposeService собирает образ одного compose-сервиса:
// свой Dockerfile/context — свой тег, готовый image — pull.
func (h *Handler) buildComposeService(ctx context.Context, task Task, svc git.ComposeService) (string, error) {
	if svc.Dockerfile != "" {
		tag := "app-" + shortID(task.DeploymentID) + "-" + svc.Name + ":latest"
		if err := h.dock.Build(ctx, svc.ContextDir, svc.Dockerfile, tag, h.buildLines(task)); err != nil {
			return "", err
		}
		return tag, nil
	}
	if svc.Image != "" {
		if err := h.dock.EnsureImage(ctx, svc.Image); err != nil {
			return "", err
		}
		return svc.Image, nil
	}
	return "", errComposeEmpty
}

func (h *Handler) buildDockerfile(ctx context.Context, task Task, contextDir, dockerfile, tag string) error {
	h.logger.Info("deploy build started", "deployment", task.DeploymentID, "dockerfile", dockerfile)
	start := time.Now()
	if err := h.dock.Build(ctx, contextDir, dockerfile, tag, h.buildLines(task)); err != nil {
		h.logger.Warn("deploy build failed", "deployment", task.DeploymentID, "error", err)
		return err
	}
	h.logger.Info("deploy build finished", "deployment", task.DeploymentID, "took", time.Since(start).Round(time.Second).String())
	return nil
}

// failChild уведомляет центр о провале одного сервис-контейнера.
func (h *Handler) failChild(ctx context.Context, task Task, parentApp, deploymentID, appName, message string, err error) error {
	ev := appEvent("deploy.failed", task, message)
	ev.DeploymentID = deploymentID
	ev.AppName = appName
	ev.ParentApp = parentApp
	h.publishEvent(ctx, ev)
	return err
}

// buildLines складывает строки сборки в buildlog и подсвечивает ERROR в логах.
func (h *Handler) buildLines(task Task) func(line string, progress bool) {
	return func(line string, progress bool) {
		h.builds.Append(task.DeploymentID, line)
		if progress {
			return
		}
		if strings.HasPrefix(line, "ERROR:") {
			h.logger.Warn("deploy build", "deployment", task.DeploymentID, "line", line)
			return
		}
		h.logger.Info("deploy build", "deployment", task.DeploymentID, "line", line)
	}
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

// failDeploy уведомляет центр о провале и возвращает ошибку дальше.
func (h *Handler) failDeploy(ctx context.Context, task Task, message string, err error) error {
	h.publishEvent(ctx, appEvent("deploy.failed", task, message))
	return err
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

func (h *Handler) deploy(ctx context.Context, task Task) error {
	h.logger.Info("deploy started", "deployment", task.DeploymentID, "app", task.AppName, "repo", task.RepoURL)
	// Прямой запуск готового образа (refresh, повторный прогон): один контейнер.
	if task.RepoURL == "" {
		image, err := h.pullImage(ctx, task.Image)
		if err != nil {
			return h.failDeploy(ctx, task, "образ не скачан: "+err.Error(), err)
		}
		return h.deploySingle(ctx, task, image, task.AppName, task.DeploymentID, task.Ports, task.ContainerName, "", "", "", "")
	}
	dir, err := h.projectDir(task)
	if err != nil {
		return h.failDeploy(ctx, task, err.Error(), err)
	}
	if err := git.EnsureCloned(ctx, task.RepoURL, dir); err != nil {
		return h.failDeploy(ctx, task, "репозиторий не склонирован: "+err.Error(), err)
	}
	dockerfile, compose := git.Detect(dir)
	if dockerfile == "" && compose == "" {
		return h.failDeploy(ctx, task, errNoDockerfile.Error(), errNoDockerfile)
	}
	// Обычный Dockerfile: один контейнер как раньше.
	if dockerfile != "" {
		tag := "app-" + shortID(task.DeploymentID) + ":latest"
		if err := h.buildDockerfile(ctx, task, dir, dockerfile, tag); err != nil {
			return h.failDeploy(ctx, task, "образ не собран: "+err.Error(), err)
		}
		return h.deploySingle(ctx, task, tag, task.AppName, task.DeploymentID, task.Ports, task.ContainerName, "", "", "", "")
	}
	return h.deployCompose(ctx, task, dir, compose)
}

// deploySingle запускает один контейнер с лимитами и сообщает центру.
// parentID/parentApp/service заполнены только для сервис-контейнеров compose.
func (h *Handler) deploySingle(ctx context.Context, task Task, image, appName, deploymentID string, ports []PortTask, customName, network, parentID, parentApp, service string) error {
	h.logger.Info("deploy image ready", "deployment", deploymentID, "image", image)
	volumes, err := h.resolveVolumes(task)
	if err != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "тома неверны: "+err.Error(), err)
	}
	mapped, err := toPortMappings(ports)
	if err != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "неверный порт", err)
	}
	name, err := sanitizeContainerName(appName, customName, deploymentID)
	if err != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, err.Error(), err)
	}
	portsJSON, _ := json.Marshal(ports)
	volumesJSON, _ := json.Marshal(task.Volumes)
	envJSON, _ := json.Marshal(task.Env)
	if err := h.dock.Remove(ctx, name); err != nil && !docker.IsMissing(err) {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "старый контейнер не удалён: "+err.Error(), err)
	}
	containerID, err := h.dock.Run(ctx, docker.RunSpec{
		Name:          name,
		Image:         image,
		Env:           task.Env,
		Ports:         mapped,
		Volumes:       volumes,
		RestartPolicy: task.Restart,
		CPUCores:      task.CPUCores,
		MemoryGB:      task.MemoryGB,
		Network:       network,
	})
	if err != nil {
		h.logger.Warn("deploy run failed", "deployment", deploymentID, "error", err)
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "запуск не удался: "+err.Error(), err)
	}
	h.logger.Info("deploy completed", "deployment", deploymentID, "container", containerID)

	_ = store.SaveDeployment(h.db, store.Deployment{
		ID:        deploymentID,
		AppID:     task.AppID,
		AppName:   appName,
		Owner:     task.Owner,
		RepoURL:   task.RepoURL,
		Name:      name,
		Image:     image,
		Container: containerID,
		Status:    "running",
		Ports:     string(portsJSON),
		Volumes:   string(volumesJSON),
		Env:       string(envJSON),
		CPUCores:  task.CPUCores,
		MemoryGB:  task.MemoryGB,
		ParentID:  parentID,
		ParentApp: parentApp,
		Service:   service,
	})

	ev := appEvent("deploy.completed", task, "приложение запущено")
	ev.DeploymentID = deploymentID
	ev.AppName = appName
	ev.Image = image
	ev.Ports = ports
	ev.ParentApp = parentApp
	h.publishEvent(ctx, ev)

	if h.onReady != nil {
		h.onReady(containerID, name)
	}
	return nil
}

// deployCompose запускает каждый сервис compose отдельным контейнером
// «приложение-сервис» в общей сети. Лимиты из заявки действуют на КАЖДЫЙ
// контейнер, пользовательские порты отдаются первому сервису.
func (h *Handler) deployCompose(ctx context.Context, task Task, dir, compose string) error {
	all, err := git.ParseComposeServices(dir, compose)
	if err != nil {
		return h.failDeploy(ctx, task, "compose не разобран: "+err.Error(), err)
	}
	svcs := all
	if len(task.Services) > 0 {
		requested := map[string]bool{}
		for _, name := range task.Services {
			requested[name] = true
		}
		filtered := make([]git.ComposeService, 0, len(task.Services))
		for _, svc := range all {
			if requested[svc.Name] {
				filtered = append(filtered, svc)
			}
		}
		svcs = filtered
		if len(svcs) == 0 {
			return h.failDeploy(ctx, task, "запрошенные сервисы не найдены в compose", errComposeEmpty)
		}
	}
	netName, _ := sanitizeContainerName(task.AppName, task.AppName+"-net", task.DeploymentID)
	if err := h.dock.EnsureNetwork(ctx, netName); err != nil {
		return h.failDeploy(ctx, task, "сеть не создана: "+err.Error(), err)
	}
	failed := 0
	for i, svc := range svcs {
		childApp := task.AppName + "-" + svc.Name
		childID := task.DeploymentID + "-" + svc.Name
		var ports []PortTask
		if i == 0 {
			ports = task.Ports
		}
		image, err := h.buildComposeService(ctx, task, svc)
		if err != nil {
			failed++
			_ = h.failChild(ctx, task, task.AppName, childID, childApp, "сервис "+svc.Name+": "+err.Error(), err)
			continue
		}
		if err := h.deploySingle(ctx, task, image, childApp, childID, ports, childApp, netName, task.DeploymentID, task.AppName, svc.Name); err != nil {
			failed++
		}
	}
	if failed == len(svcs) {
		return h.failDeploy(ctx, task, "ни один сервис не запустился", errComposeEmpty)
	}
	return nil
}

// loadDeployment достаёт развёртывание из SQLite.
// Неизвестный id — постоянная ошибка (ретрай не поможет), центр уведомляется.
func (h *Handler) loadDeployment(ctx context.Context, task Task, what string) (*store.Deployment, error) {
	dep, err := store.GetDeployment(h.db, task.DeploymentID)
	if err == nil {
		return dep, nil
	}
	if errors.Is(err, store.ErrNoDeployment) {
		return nil, h.missingDeployment(ctx, task, what)
	}
	return nil, err
}

func (h *Handler) refresh(ctx context.Context, task Task) error {
	dep, err := h.loadDeployment(ctx, task, "обновление")
	if err != nil {
		return err
	}
	owner := dep.Owner
	if owner == "" {
		owner = task.Owner
	}
	var ports []PortTask
	_ = json.Unmarshal([]byte(dep.Ports), &ports)
	var volumes []VolumeTask
	_ = json.Unmarshal([]byte(dep.Volumes), &volumes)
	var env map[string]string
	_ = json.Unmarshal([]byte(dep.Env), &env)
	// Сервис-контейнер compose пересобирается через родительскую задачу,
	// чтобы clone/detect шли в правильную папку проекта.
	if dep.ParentID != "" && dep.Service != "" {
		parentApp := dep.ParentApp
		if parentApp == "" {
			parentApp = dep.AppName
		}
		return h.deploy(ctx, Task{
			DeploymentID: dep.ParentID,
			AppID:        dep.AppID,
			AppName:      parentApp,
			Owner:        owner,
			RepoURL:      dep.RepoURL,
			Services:     []string{dep.Service},
			Ports:        ports,
			Volumes:      volumes,
			Env:          env,
			Restart:      "unless-stopped",
			CPUCores:     dep.CPUCores,
			MemoryGB:     dep.MemoryGB,
		})
	}
	return h.deploy(ctx, Task{
		DeploymentID:  dep.ID,
		AppID:         dep.AppID,
		AppName:       dep.AppName,
		Owner:         owner,
		RepoURL:       dep.RepoURL,
		Image:         dep.Image,
		ContainerName: dep.Name,
		Ports:         ports,
		Volumes:       volumes,
		Env:           env,
		Restart:       "unless-stopped",
		CPUCores:      dep.CPUCores,
		MemoryGB:      dep.MemoryGB,
	})
}

func (h *Handler) stop(ctx context.Context, task Task) error {
	dep, err := h.loadDeployment(ctx, task, "остановка")
	if err != nil {
		return err
	}
	if err := h.dock.Stop(ctx, dep.Container); err != nil {
		h.logger.Warn("stop failed", "deployment", task.DeploymentID, "error", err)
		h.publishEvent(ctx, appEvent("op.failed", task, "остановка не удалась: "+err.Error()))
		return queue.Permanent(err)
	}
	_ = store.UpdateDeploymentStatus(h.db, task.DeploymentID, "stopped", dep.Container)
	h.publishEvent(ctx, appEvent("container.stopped", task, "приложение остановлено"))
	if h.onStopped != nil {
		h.onStopped(dep.Container)
	}
	return nil
}

func (h *Handler) restart(ctx context.Context, task Task) error {
	dep, err := h.loadDeployment(ctx, task, "перезапуск")
	if err != nil {
		return err
	}
	if err := h.dock.Restart(ctx, dep.Container); err != nil {
		h.logger.Warn("restart failed", "deployment", task.DeploymentID, "error", err)
		h.publishEvent(ctx, appEvent("op.failed", task, "перезапуск не удался: "+err.Error()))
		return queue.Permanent(err)
	}
	_ = store.UpdateDeploymentStatus(h.db, task.DeploymentID, "running", dep.Container)
	h.publishEvent(ctx, appEvent("container.restarted", task, "приложение перезапущено"))
	return nil
}

func (h *Handler) remove(ctx context.Context, task Task) error {
	dep, err := store.GetDeployment(h.db, task.DeploymentID)
	if err != nil {
		if errors.Is(err, store.ErrNoDeployment) {
			// Idempotent delete: nothing stored locally means there is
			// nothing to stop — report success so central drops the record.
			h.logger.Info("remove of unknown deployment, treating as done", "deployment", task.DeploymentID)
			h.publishEvent(ctx, appEvent("container.removed", task, "приложение удалено"))
			return nil
		}
		return err
	}
	if err := h.dock.Remove(ctx, dep.Container); err != nil && !docker.IsMissing(err) {
		h.logger.Warn("remove failed", "deployment", task.DeploymentID, "error", err)
		h.publishEvent(ctx, appEvent("op.failed", task, "удаление не удалось: "+err.Error()))
		return queue.Permanent(err)
	}
	_ = store.DeleteDeployment(h.db, task.DeploymentID)

	// Delete project directory on worker (git repo, build artifacts)
	projDir, err := fsx.ProjectDir(h.workspace, dep.Owner, dep.AppName)
	if err == nil {
		if err := os.RemoveAll(projDir); err != nil {
			h.logger.Warn("failed to remove project dir", "dir", projDir, "error", err)
		} else {
			h.logger.Info("project directory removed", "dir", projDir)
		}
	}

	h.publishEvent(ctx, appEvent("container.removed", task, "приложение удалено"))
	if h.onStopped != nil {
		h.onStopped(dep.Container)
	}
	return nil
}

func (h *Handler) volumeRemove(ctx context.Context, task Task) error {
	name, err := sanitizeVolumeName(task.VolumeName)
	if err != nil {
		return queue.Permanent(err)
	}
	return h.dock.VolumeRemove(ctx, name)
}

func (h *Handler) reboot(ctx context.Context) error {
	if !canReboot() {
		h.publishEvent(ctx, Event{Type: "server.reboot_failed", Message: "перезагрузка не поддерживается"})
		return errRebootUnsupported
	}
	h.publishEvent(ctx, Event{Type: "server.rebooting", Message: "сервер перезагружается"})
	go delayedReboot(h.logger)
	return nil
}
