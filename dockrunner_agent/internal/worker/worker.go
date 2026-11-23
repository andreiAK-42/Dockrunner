package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"

	"dockrunner_agent/internal/buildlog"
	"dockrunner_agent/internal/docker"
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

// UnmarshalJSON accepts both the deploy shape ({host_port, container_port})
// and the API/spec shape ({host, container}) used by older recreate records.
func (p *PortTask) UnmarshalJSON(data []byte) error {
	var raw struct {
		Host         int `json:"host_port"`
		Container    int `json:"container_port"`
		HostAPI      int `json:"host"`
		ContainerAPI int `json:"container"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Host = raw.Host
	if p.Host == 0 {
		p.Host = raw.HostAPI
	}
	p.Container = raw.Container
	if p.Container == 0 {
		p.Container = raw.ContainerAPI
	}
	return nil
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
	Services  []string `json:"services"`
	BuildMode string   `json:"build_mode"`
	// Branch — git branch to clone (required if repo_url is set).
	Branch string `json:"branch"`
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
	Branches           []string `json:"branches,omitempty"`
	// DetectedEnvVars — переменные окружения из .env/.env.example
	DetectedEnvVars map[string]string `json:"detected_env_vars,omitempty"`
	// DetectedPorts — порты из Dockerfile EXPOSE и docker-compose ports/expose
	DetectedPorts []DetectedPort `json:"detected_ports,omitempty"`
	// ParentApp — головное приложение для событий сервис-контейнеров
	// (нужно центру, чтобы снять oplock родительской операции).
	ParentApp string     `json:"parent_app,omitempty"`
	Ports     []PortTask `json:"ports"`
	Message   string     `json:"msg"`
	Time      string     `json:"time"`
}

type DetectedPort struct {
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Source        string `json:"source"`            // "dockerfile" or "compose"
	Service       string `json:"service,omitempty"` // service name for compose
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

// publishEvent отправляет событие в очередь events. Ошибка возвращается
// вызывающему: терминальное событие (container.removed и т.п.) обязано
// доехать до централи, иначе та ждёт его бесконечно — лучше перепопытка.
func (h *Handler) publishEvent(ctx context.Context, event Event) error {
	if h.queue == nil {
		return errors.New("queue unavailable")
	}
	event.AgentID = h.agentID
	event.Time = time.Now().UTC().Format(time.RFC3339)
	body, _ := json.Marshal(event)
	if err := h.queue.Publish(ctx, queue.QueueEvents, body); err != nil {
		h.logger.Warn("publish event failed", "type", event.Type, "deployment", event.DeploymentID, "error", err)
		return err
	}
	h.logger.Info("event published", "type", event.Type, "deployment", event.DeploymentID)
	return nil
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
// Если само уведомление не ушло — возвращаем ошибку: лучше перепопытка,
// чем молчаливый ответ, после которого централь ждёт событие вечно.
func (h *Handler) missingDeployment(ctx context.Context, task Task, what string) error {
	h.logger.Warn("unknown local deployment", "type", task.Type, "deployment", task.DeploymentID)
	if err := h.publishEvent(ctx, appEvent("op.failed", task, what+": развёртывание не найдено на узле — сначала запустите приложение")); err != nil {
		return err
	}
	return queue.Permanent(store.ErrNoDeployment)
}
