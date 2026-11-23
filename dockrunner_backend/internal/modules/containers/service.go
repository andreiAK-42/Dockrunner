package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
	"dockrunner_backend/internal/modules/agents"
	"dockrunner_backend/internal/modules/volumes"
	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/internal/realtime"
	"dockrunner_backend/pkg/queue"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrNoQueue     = errors.New("queue unavailable")
	ErrUnknownNode = errors.New("узел не присылал heartbeat — проверьте, что агент запущен")
	ErrNodeOffline = errors.New("узел не в сети — нет свежего heartbeat")
)

type DetectedPort struct {
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	Source        string `json:"source"`            // "dockerfile" or "compose"
	Service       string `json:"service,omitempty"` // service name for compose
}

type Check struct {
	ID                 string            `json:"id"`
	AppName            string            `json:"app_name"`
	RepoURL            string            `json:"repo_url"`
	NodeID             string            `json:"node_id"`
	Status             string            `json:"status"`
	DetectedDockerfile bool              `json:"detected_dockerfile"`
	ComposeFile        string            `json:"compose_file"`
	Services           []string          `json:"services"`
	Branches           []string          `json:"branches,omitempty"`
	DetectedEnvVars    map[string]string `json:"detected_env_vars,omitempty"`
	DetectedPorts      []DetectedPort    `json:"detected_ports,omitempty"`
	Error              string            `json:"error"`
	owner              string
	expires            time.Time
}

type ContainerStore interface {
	GetContainer(ctx context.Context, id string) (*admin.ContainerRecord, error)
	FindLatestByApp(ctx context.Context, owner, appName string) (*admin.ContainerRecord, error)
	ListByOwner(ctx context.Context, owner string) ([]admin.ContainerRecord, error)
	ListByNode(ctx context.Context, nodeID string) ([]admin.ContainerRecord, error)
	ListContainers(ctx context.Context) ([]admin.ContainerRecord, error)
	SetContainerStatus(ctx context.Context, id, status string) error
	UpsertContainer(ctx context.Context, c admin.ContainerRecord) error
}

// PortsPolicy is the allowed host-port range for user containers.
type PortsPolicy struct {
	Min int
	Max int
}

func normalizePorts(p PortsPolicy) PortsPolicy {
	if p.Min < 1 {
		p.Min = 1
	}
	if p.Max > 65535 {
		p.Max = 65535
	}
	if p.Min > p.Max {
		p.Max = p.Min
	}
	return p
}

type Service struct {
	mu         sync.Mutex
	checks     map[string]*Check
	ops        *oplock.Tracker
	nodeLocks  map[string]*sync.Mutex
	store      ContainerStore
	agents     *agents.Repository
	volumes    *volumes.Service
	ports      PortsPolicy
	agentToken string
	queue      *queue.Client
	logger     *zap.Logger
	hub        *realtime.Hub
}

func NewService(store ContainerStore, agents *agents.Repository, vols *volumes.Service, agentToken string, queue *queue.Client, logger *zap.Logger, ops *oplock.Tracker, ports PortsPolicy, hub *realtime.Hub) *Service {
	if ops == nil {
		ops = &oplock.Tracker{}
	}
	return &Service{
		checks:     map[string]*Check{},
		ops:        ops,
		nodeLocks:  map[string]*sync.Mutex{},
		store:      store,
		agents:     agents,
		volumes:    vols,
		ports:      normalizePorts(ports),
		agentToken: agentToken,
		queue:      queue,
		logger:     logger,
		hub:        hub,
	}
}

// publishContainer pushes a container state change to its owner and admins.
func (s *Service) publishContainer(owner string, rec admin.ContainerRecord) {
	s.hub.Emit("container.updated", realtime.TopicContainers, owner, rec)
}

// publishCheck pushes a repository check result to the owner (and admins).
func (s *Service) publishCheck(owner string, check *Check) {
	s.hub.Emit("check.updated", realtime.TopicChecks, owner, check)
}

// ReleaseOp frees the operation lock for owner/app. Called when a terminal
// event (deploy.completed/failed, op.failed, container.*) arrives.
func (s *Service) ReleaseOp(owner, app string) {
	if owner == "" && app == "" {
		return
	}
	s.ops.Release(owner, app)
}

// lockNode serializes port validation + task publish per worker node,
// so two concurrent runs cannot grab the same host port.
func (s *Service) lockNode(nodeID string) func() {
	s.mu.Lock()
	l, ok := s.nodeLocks[nodeID]
	if !ok {
		l = &sync.Mutex{}
		s.nodeLocks[nodeID] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// busyPortsByNode собирает занятые host-порты узла в карту порт -> приложение.
func (s *Service) busyPortsByNode(ctx context.Context, nodeID string) (map[int]string, error) {
	recs, err := s.store.ListByNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	busy := map[int]string{}
	for _, rec := range recs {
		if rec.Status == "stopped" || rec.Status == "error" {
			continue
		}
		for _, rp := range rec.Ports {
			busy[rp.HostPort] = rec.AppName
		}
	}
	return busy, nil
}

// checkPorts validates the host-port range and busy ports on the node.
// Records in stopped/error state hold no ports and are skipped.
func (s *Service) checkPorts(ctx context.Context, nodeID string, ports []PortSpec) error {
	for _, p := range ports {
		if p.HostPort < s.ports.Min || p.HostPort > s.ports.Max {
			return fmt.Errorf("порт %d вне разрешённого диапазона %d–%d", p.HostPort, s.ports.Min, s.ports.Max)
		}
	}
	if len(ports) == 0 {
		return nil
	}
	busyPorts, err := s.busyPortsByNode(ctx, nodeID)
	if err != nil {
		return err
	}
	for _, p := range ports {
		holder, busy := busyPorts[p.HostPort]
		if !busy {
			continue
		}
		if holder == "" {
			holder = "другим приложением"
		} else {
			holder = "приложением «" + holder + "»"
		}
		return fmt.Errorf("порт %d уже занят %s на этом узле", p.HostPort, holder)
	}
	return nil
}

// validateResources проверяет, что запрошенные ресурсы не превышают емкость узла.
func (s *Service) validateResources(ctx context.Context, nodeID string, cpuCores, memoryGB, diskGB float64) error {
	agents, err := s.agents.List(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}
	for _, a := range agents {
		if a.ID == nodeID {
			// Check CPU (convert cpuCores to cores, agent has CPUTotal in cores)
			if cpuCores > float64(a.CPUTotal) {
				return fmt.Errorf("requested CPU %.2f cores exceeds node capacity %d cores", cpuCores, a.CPUTotal)
			}
			// Check Memory
			if memoryGB > a.MemTotalGB {
				return fmt.Errorf("requested memory %.2f GB exceeds node capacity %.2f GB", memoryGB, a.MemTotalGB)
			}
			// Check Disk
			if diskGB > a.DiskTotalGB {
				return fmt.Errorf("requested disk %.2f GB exceeds node capacity %.2f GB", diskGB, a.DiskTotalGB)
			}
			return nil
		}
	}
	return fmt.Errorf("node %s not found", nodeID)
}

func (s *Service) Mine(ctx context.Context, owner string) ([]admin.ContainerRecord, error) {
	return s.store.ListByOwner(ctx, owner)
}

func (s *Service) agentGet(ctx context.Context, nodeID, suffix string) ([]byte, error) {
	apiURL, err := s.agents.AgentApiURL(ctx, nodeID)
	if err != nil {
		if errors.Is(err, agents.ErrNoAPIURL) {
			return nil, err
		}
		return nil, fmt.Errorf("agent unreachable")
	}
	if apiURL == "" {
		return nil, agents.ErrNoAPIURL
	}
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, trimSlash(apiURL)+suffix, nil)
	if err != nil {
		return nil, err
	}
	if s.agentToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.agentToken)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent unreachable")
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("agent error: %s", http.StatusText(res.StatusCode))
	}
	return body, nil
}

func (s *Service) agentPost(ctx context.Context, nodeID, suffix string, payload any) ([]byte, error) {
	apiURL, err := s.agents.AgentApiURL(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if apiURL == "" {
		return nil, agents.ErrNoAPIURL
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, trimSlash(apiURL)+suffix, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.agentToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.agentToken)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("agent unreachable: %w", err)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("agent error: %s", strings.TrimSpace(string(data)))
	}
	return data, nil
}

// agentContainerIDs — id деплоев, которые имеет смысл спросить у агента
// для этой записи: сам id, родительский деплой compose-сервиса и другие
// деплои того же приложения на том же узле. Последние спасают ситуацию,
// когда новый деплой не дошёл до создания контейнера, а контейнер прошлого
// деплоя этого приложения продолжает работать.
func (s *Service) agentContainerIDs(ctx context.Context, rec *admin.ContainerRecord) []string {
	const maxIDs = 5
	ids := []string{rec.ID}
	seen := map[string]bool{rec.ID: true}
	add := func(id string) {
		if id == "" || seen[id] || len(ids) >= maxIDs {
			return
		}
		seen[id] = true
		ids = append(ids, id)
	}
	// Составные id сервисов compose: <uuid>-<service> (uuid — 36 символов).
	if len(rec.ID) > 36 && rec.ID[36] == '-' {
		add(rec.ID[:36])
	}
	recs, err := s.store.ListByOwner(ctx, rec.OwnerLogin)
	if err != nil {
		return ids
	}
	for _, sibling := range recs {
		if len(ids) >= maxIDs {
			break
		}
		if sibling.ID == rec.ID || sibling.NodeID != rec.NodeID || sibling.AppName != rec.AppName {
			continue
		}
		add(sibling.ID)
	}
	return ids
}

// syncStatusFromSpec сверяет статус из ответа агента с записью в БД и
// обновляет её, если они разошлись (например, агент восстановил запись
// о живом контейнере, а в БД остался "stopped").
func (s *Service) syncStatusFromSpec(ctx context.Context, rec *admin.ContainerRecord, body []byte) {
	// Агент отдаёт spec плоским объектом; обработчик заворачивает его в
	// {"data": ...}. Поддерживаем оба варианта, чтобы статус не «залипал».
	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return
	}
	status := payload.Status
	if status == "" {
		status = payload.Data.Status
	}
	if status == "" || status == rec.Status {
		return
	}
	// Пока идёт сборка/перезапуск, статус ведёт агент: spec в это время
	// может описывать ещё старый (работающий) контейнер.
	if rec.Status == "building" {
		return
	}
	if err := s.store.SetContainerStatus(ctx, rec.ID, status); err != nil {
		return
	}
	rec.Status = status
	s.publishContainer(rec.OwnerLogin, *rec)
}

// recreatePort accepts both port shapes used by the API and update requests:
// {host, container} and {host_port, container_port}.
type recreatePort struct {
	Host          int `json:"host"`
	Container     int `json:"container"`
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

func (p recreatePort) host() int {
	if p.Host != 0 {
		return p.Host
	}
	return p.HostPort
}

func (p recreatePort) container() int {
	if p.Container != 0 {
		return p.Container
	}
	return p.ContainerPort
}

func samePortMappings(current []admin.PortMapping, provided []recreatePort) bool {
	if len(current) != len(provided) {
		return false
	}
	left := make(map[[2]int]int, len(current))
	for _, p := range current {
		left[[2]int{p.HostPort, p.ContainerPort}]++
	}
	for _, p := range provided {
		key := [2]int{p.host(), p.container()}
		if left[key] == 0 {
			return false
		}
		left[key]--
	}
	return true
}

func (s *Service) RecreateContainer(ctx context.Context, owner, id string, spec map[string]any) ([]byte, error) {
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return nil, ErrNotFound
	}
	if err := s.ops.Acquire(rec.OwnerLogin, rec.AppName, "recreate"); err != nil {
		return nil, err
	}
	defer s.ops.Release(rec.OwnerLogin, rec.AppName)
	// Fundamental parameters of a running container are immutable: a recreate
	// may change runtime settings, but moving the app to another port or image
	// has to go through a full update. Validate before the agent touches it.
	var updated struct {
		Image         string             `json:"image"`
		Ports         []recreatePort     `json:"ports"`
		Volumes       []admin.VolumeSpec `json:"volumes"`
		Env           map[string]string  `json:"env"`
		RestartPolicy string             `json:"restart_policy"`
	}
	if raw, marshalErr := json.Marshal(spec); marshalErr == nil {
		_ = json.Unmarshal(raw, &updated)
	}
	if _, provided := spec["ports"]; provided && !samePortMappings(rec.Ports, updated.Ports) {
		return nil, errors.New("порты запущенного контейнера изменить нельзя")
	}
	if updated.Image != "" && rec.Image != "" && updated.Image != rec.Image {
		return nil, errors.New("образ запущенного контейнера изменить нельзя — используйте обновление")
	}
	responseBody, err := s.agentPost(ctx, rec.NodeID, "/containers/"+id+"/recreate", spec)
	if err != nil {
		return nil, err
	}
	// Keep the central list in sync with the live container specification.
	if updated.Image != "" {
		rec.Image = updated.Image
	}
	rec.Status = "running"
	if _, provided := spec["ports"]; provided {
		rec.Ports = make([]admin.PortMapping, 0, len(updated.Ports))
		for _, p := range updated.Ports {
			rec.Ports = append(rec.Ports, admin.PortMapping{HostPort: p.host(), ContainerPort: p.container()})
		}
	}
	// Update volumes, env, restart_policy from spec if provided
	if len(updated.Volumes) > 0 {
		rec.Volumes = updated.Volumes
	}
	if len(updated.Env) > 0 {
		rec.Env = updated.Env
	}
	if updated.RestartPolicy != "" {
		rec.RestartPolicy = updated.RestartPolicy
	}
	if err := s.store.UpsertContainer(ctx, *rec); err != nil {
		s.logger.Warn("container recreated but central record update failed", zap.String("container", id), zap.Error(err))
	}
	s.publishContainer(rec.OwnerLogin, *rec)
	return responseBody, nil
}

func (s *Service) ContainerSpec(ctx context.Context, owner, id string) ([]byte, error) {
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return nil, ErrNotFound
	}

	// Спрашиваем агента по id записи, затем по id других деплоев того же
	// приложения (и по родителю compose-сервиса). app_name/owner позволяют
	// агенту найти работающий контейнер, даже если он создан другим деплоем.
	appQuery := "?app_name=" + url.QueryEscape(rec.AppName) + "&owner=" + url.QueryEscape(rec.OwnerLogin)
	var firstErr error
	for _, agentID := range s.agentContainerIDs(ctx, rec) {
		body, reqErr := s.agentGet(ctx, rec.NodeID, "/containers/"+agentID+"/spec"+appQuery)
		if reqErr != nil {
			if firstErr == nil {
				firstErr = reqErr
			}
			// Другие id пробуем только если агент ответил «не найдено»:
			// при недоступном узле это лишь умножит таймауты.
			if !agentNotFound(reqErr) {
				break
			}
			continue
		}
		s.syncStatusFromSpec(ctx, rec, body)
		return body, nil
	}
	err = firstErr
	if err == nil {
		err = errors.New("agent returned no container spec")
	}
	if agentNotFound(err) {
		// Пока идёт сборка, отсутствие контейнера на узле — норма, а не
		// признак остановки: статус не трогаем.
		if rec.Status != "building" {
			// Container doesn't exist on agent anymore - update DB status
			_ = s.store.SetContainerStatus(ctx, id, "stopped")
			rec.Status = "stopped"
			s.publishContainer(rec.OwnerLogin, *rec)
		}

		// Build spec response from DB record. Формат должен совпадать с
		// ответом агента (host/container), иначе фронт получает undefined.
		dbPorts := make([]struct {
			Host      int `json:"host"`
			Container int `json:"container"`
		}, 0, len(rec.Ports))
		for _, p := range rec.Ports {
			dbPorts = append(dbPorts, struct {
				Host      int `json:"host"`
				Container int `json:"container"`
			}{Host: p.HostPort, Container: p.ContainerPort})
		}
		spec := map[string]any{
			"id":             rec.ID,
			"name":           rec.AppName,
			"image":          rec.Image,
			"status":         rec.Status,
			"ports":          dbPorts,
			"volumes":        rec.Volumes,
			"env":            rec.Env,
			"entrypoint":     []string{},
			"cmd":            []string{},
			"restart_policy": rec.RestartPolicy,
			"network":        "",
			"working_dir":    "",
			"user":           "",
		}
		return json.Marshal(spec)
	}

	return nil, err
}

type ContainerStats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemMB      float64 `json:"mem_mb"`
}

func (s *Service) ContainerStats(ctx context.Context, owner, id string) (*ContainerStats, error) {
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return nil, ErrNotFound
	}

	// Спрашиваем агента по id записи и по другим деплоям этого приложения.
	appQuery := "?app_name=" + url.QueryEscape(rec.AppName) + "&owner=" + url.QueryEscape(rec.OwnerLogin)
	var firstErr error
	for _, agentID := range s.agentContainerIDs(ctx, rec) {
		body, reqErr := s.agentGet(ctx, rec.NodeID, "/containers/"+agentID+"/stats"+appQuery)
		if reqErr != nil {
			if firstErr == nil {
				firstErr = reqErr
			}
			if !agentNotFound(reqErr) {
				break
			}
			continue
		}
		var stats ContainerStats
		if err := json.Unmarshal(body, &stats); err != nil {
			return nil, err
		}
		return &stats, nil
	}
	err = firstErr
	if err == nil {
		err = errors.New("agent returned no container stats")
	}

	// If all attempts failed, check if it's a 404 (container not found on agent)
	// In that case, update the DB status to "stopped" and return zeros (container is stopped)
	if agentNotFound(err) {
		// Во время сборки контейнера ещё нет — это не «stopped».
		if rec.Status != "building" {
			// Container doesn't exist on agent anymore - update DB status
			_ = s.store.SetContainerStatus(ctx, id, "stopped")
		}
		return &ContainerStats{CPUPercent: 0, MemMB: 0}, nil
	}

	return nil, err
}

// ContainerSizeInfo represents disk usage info for a container from the agent.
type ContainerSizeInfo struct {
	DeploymentID string `json:"deployment_id"`
	ContainerID  string `json:"container_id"`
	SizeBytes    int64  `json:"size_bytes"`
}

// GetContainerSizes retrieves disk usage (writable layer size) for all containers on a node.
func (s *Service) GetContainerSizes(ctx context.Context, nodeID string) ([]ContainerSizeInfo, error) {
	body, err := s.agentGet(ctx, nodeID, "/containers/sizes")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Data []ContainerSizeInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload.Data, nil
}

func (s *Service) ContainerLogs(ctx context.Context, owner, id string, tail int) (string, error) {
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return "", ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return "", ErrNotFound
	}
	if tail <= 0 || tail > 10000 {
		tail = 200
	}

	// Спрашиваем агента по id записи и по другим деплоям этого приложения:
	// работающий контейнер мог остаться от предыдущего деплоя.
	appQuery := "&app_name=" + url.QueryEscape(rec.AppName) + "&owner=" + url.QueryEscape(rec.OwnerLogin)
	var firstErr error
	for _, agentID := range s.agentContainerIDs(ctx, rec) {
		body, reqErr := s.agentGet(ctx, rec.NodeID, fmt.Sprintf("/containers/%s/logs?tail=%d", agentID, tail)+appQuery)
		if reqErr != nil {
			if firstErr == nil {
				firstErr = reqErr
			}
			if !agentNotFound(reqErr) {
				break
			}
			continue
		}
		var payload struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return "", err
		}
		return payload.Data, nil
	}
	err = firstErr
	if err == nil {
		err = errors.New("agent returned no container logs")
	}

	// If all attempts failed, check if it's a 404 (container not found on agent)
	// In that case, update the DB status to "stopped" and return empty logs (container is stopped)
	if agentNotFound(err) {
		// Во время сборки контейнера ещё нет — статус не трогаем.
		if rec.Status == "building" {
			return "Контейнер ещё не создан — идёт сборка или запуск.", nil
		}
		// Container doesn't exist on agent anymore - update DB status
		_ = s.store.SetContainerStatus(ctx, id, "stopped")
		return "Контейнер не найден на узле (возможно, был удален). Статус обновлен на 'stopped'.", nil
	}

	return "", err
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

// agentNotFound сообщает, что агент ответил 404 (деплой/контейнер не найден),
// а не что узел недоступен или Docker вернул другую ошибку.
func agentNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") || strings.Contains(msg, "not found")
}

func (s *Service) StartCheck(ctx context.Context, owner, appName, repoURL, nodeID, branch string) (*Check, error) {
	if s.queue == nil {
		return nil, ErrNoQueue
	}
	if err := s.requireOnlineNode(ctx, nodeID); err != nil {
		return nil, err
	}
	if err := s.ops.Acquire(owner, appName, "check"); err != nil {
		return nil, err
	}
	check := &Check{
		ID:      uuid.NewString(),
		AppName: appName,
		RepoURL: repoURL,
		NodeID:  nodeID,
		Status:  "checking",
		owner:   owner,
		expires: time.Now().Add(10 * time.Minute),
	}
	s.mu.Lock()
	s.prune()
	s.checks[check.ID] = check
	s.mu.Unlock()
	body, _ := json.Marshal(map[string]string{
		"type":          "prepare",
		"deployment_id": check.ID,
		"app_name":      appName,
		"owner":         owner,
		"repo_url":      repoURL,
		"branch":        branch,
	})
	if err := s.queue.Publish(ctx, "agent."+nodeID, body); err != nil {
		s.mu.Lock()
		delete(s.checks, check.ID)
		s.mu.Unlock()
		s.ops.Release(owner, appName)
		return nil, err
	}
	s.logger.Info("prepare task published", zap.String("node", nodeID), zap.String("check", check.ID))
	s.publishCheck(owner, check)
	return check, nil
}

func (s *Service) requireOnlineNode(ctx context.Context, nodeID string) error {
	agents, err := s.agents.List(ctx)
	if err != nil {
		return err
	}
	if len(agents) == 0 {
		return nil
	}
	for _, a := range agents {
		if a.ID == nodeID {
			if time.Since(a.LastSeen) > 2*time.Minute {
				return ErrNodeOffline
			}
			return nil
		}
	}
	return ErrUnknownNode
}

func (s *Service) GetCheck(id string) (*Check, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	check, ok := s.checks[id]
	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(check.expires) {
		delete(s.checks, id)
		return nil, ErrNotFound
	}
	return check, nil
}

func (s *Service) ResolveCheck(deploymentID string, ok bool, dockerfile bool, composeFile string, services []string, branches []string, detectedEnvVars map[string]string, detectedPorts []DetectedPort, errMsg string) {
	s.mu.Lock()
	check, found := s.checks[deploymentID]
	if !found {
		s.mu.Unlock()
		return
	}
	if ok {
		check.Status = "ready"
		check.DetectedDockerfile = dockerfile
		check.ComposeFile = composeFile
		check.Services = services
		check.Branches = branches
		check.DetectedEnvVars = detectedEnvVars
		check.DetectedPorts = detectedPorts
	} else {
		check.Status = "failed"
		check.Error = errMsg
	}
	owner, appName := check.owner, check.AppName
	s.mu.Unlock()
	s.ops.Release(owner, appName)
	s.publishCheck(owner, check)
}

func (s *Service) prune() {
	now := time.Now()
	for id, check := range s.checks {
		if now.After(check.expires) {
			delete(s.checks, id)
			s.ops.Release(check.owner, check.AppName)
		}
	}
}

type PortSpec struct {
	HostPort      int `json:"host_port"`
	ContainerPort int `json:"container_port"`
}

type VolumeSpec struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
}

type RunInput struct {
	AppName       string            `json:"app_name"`
	RepoURL       string            `json:"repo_url"`
	Image         string            `json:"image"`
	ContainerName string            `json:"container_name"`
	NodeID        string            `json:"node_id"`
	Ports         []PortSpec        `json:"ports"`
	Volumes       []VolumeSpec      `json:"volumes"`
	Env           map[string]string `json:"env"`
	CPUCores      float64           `json:"cpu_cores"`
	MemoryGB      float64           `json:"memory_gb"`
	DiskGB        float64           `json:"disk_gb"`
	DurationHours float64           `json:"duration_hours"`
	// Branch — git branch to deploy (required when repo_url is set)
	Branch string `json:"branch"`
	// Services — сервисы compose (пусто = один контейнер).
	// Каждый сервис станет записью «приложение-сервис» с теми же лимитами.
	Services  []string `json:"services"`
	BuildMode string   `json:"build_mode"`
}

func (s *Service) Run(ctx context.Context, owner string, in RunInput) (string, error) {
	if s.queue == nil {
		return "", ErrNoQueue
	}
	if in.RepoURL == "" && in.Image == "" {
		return "", errors.New("repo_url or image is required")
	}
	if err := s.requireOnlineNode(ctx, in.NodeID); err != nil {
		return "", err
	}
	// For multi-service, limits apply per service
	serviceCount := 1
	if len(in.Services) > 0 {
		serviceCount = len(in.Services)
	}
	if err := s.validateResources(ctx, in.NodeID, in.CPUCores*float64(serviceCount), in.MemoryGB*float64(serviceCount), in.DiskGB*float64(serviceCount)); err != nil {
		return "", err
	}
	unlockNode := s.lockNode(in.NodeID)
	defer unlockNode()
	if err := s.ops.Acquire(owner, in.AppName, "deploy"); err != nil {
		return "", err
	}
	if err := s.checkPorts(ctx, in.NodeID, in.Ports); err != nil {
		s.ops.Release(owner, in.AppName)
		return "", err
	}
	id := uuid.NewString()
	var expiresAt *time.Time
	if in.DurationHours > 0 {
		t := time.Now().Add(time.Duration(in.DurationHours * float64(time.Hour)))
		expiresAt = &t
	}
	body, _ := json.Marshal(map[string]any{
		"type":           "deploy",
		"deployment_id":  id,
		"app_name":       in.AppName,
		"owner":          owner,
		"repo_url":       in.RepoURL,
		"image":          in.Image,
		"container_name": in.ContainerName,
		"ports":          in.Ports,
		"volumes":        in.Volumes,
		"env":            in.Env,
		"restart_policy": "unless-stopped",
		"cpu_cores":      in.CPUCores,
		"memory_gb":      in.MemoryGB,
		"disk_gb":        in.DiskGB,
		"duration_hours": in.DurationHours,
		"services":       in.Services,
		"build_mode":     in.BuildMode,
		"branch":         in.Branch,
	})
	if err := s.queue.Publish(ctx, "agent."+in.NodeID, body); err != nil {
		s.ops.Release(owner, in.AppName)
		return "", err
	}
	// Records appear in /mine immediately as "building" — the real image
	// name and running status arrive with deploy.completed per container.
	// Мультисервис: по записи «приложение-сервис», порты — первому сервису.
	ports := make([]admin.PortMapping, 0, len(in.Ports))
	for _, p := range in.Ports {
		ports = append(ports, admin.PortMapping{HostPort: p.HostPort, ContainerPort: p.ContainerPort})
	}
	records := []admin.ContainerRecord{{
		ID: id, AppName: in.AppName, OwnerLogin: owner,
		Image: in.Image, Status: "building", NodeID: in.NodeID, Ports: ports,
		CPUCores:  in.CPUCores,
		MemoryGB:  in.MemoryGB,
		DiskGB:    in.DiskGB,
		ExpiresAt: expiresAt,
	}}
	if len(in.Services) > 0 {
		records = records[:0]
		for i, svc := range in.Services {
			childPorts := ports
			if i > 0 {
				childPorts = []admin.PortMapping{}
			}
			records = append(records, admin.ContainerRecord{
				ID: id + "-" + svc, AppName: in.AppName + "-" + svc, OwnerLogin: owner,
				Image: in.Image, Status: "building", NodeID: in.NodeID, Ports: childPorts,
				CPUCores:  in.CPUCores,
				MemoryGB:  in.MemoryGB,
				DiskGB:    in.DiskGB,
				ExpiresAt: expiresAt,
			})
		}
	}
	for _, rec := range records {
		if err := s.store.UpsertContainer(ctx, rec); err != nil {
			s.logger.Warn("run record not saved", zap.String("deployment", rec.ID), zap.Error(err))
			continue
		}
		s.publishContainer(owner, rec)
	}
	named := make([]volumes.NamedVolume, 0, len(in.Volumes))
	for _, v := range in.Volumes {
		named = append(named, volumes.NamedVolume{Name: v.Name, ContainerPath: v.ContainerPath})
	}
	s.volumes.TrackFromRun(ctx, owner, in.NodeID, named)
	return id, nil
}

func (s *Service) RefreshByApp(ctx context.Context, owner, appName string) (string, error) {
	if s.queue == nil {
		return "", ErrNoQueue
	}
	rec, err := s.store.FindLatestByApp(ctx, owner, appName)
	if err != nil {
		return "", ErrNotFound
	}
	if err := s.ops.Acquire(owner, appName, "refresh"); err != nil {
		return "", err
	}
	// Обновление compose-проекта пересобирает весь проект, а не один сервис.
	// Поэтому «building» должны получить все его сервисы: иначе соседние
	// контейнеры в панели остаются «running», хотя агент их уже пересоздаёт.
	records, err := s.projectRecords(ctx, owner, rec)
	if err != nil {
		s.ops.Release(owner, appName)
		return "", err
	}
	for _, sibling := range records {
		if err := s.store.SetContainerStatus(ctx, sibling.ID, "building"); err != nil {
			s.ops.Release(owner, appName)
			return "", err
		}
	}
	body, _ := json.Marshal(map[string]string{
		"type":          "refresh",
		"deployment_id": rec.ID,
		"app_name":      appName,
		"owner":         owner,
	})
	if err := s.queue.Publish(ctx, "agent."+rec.NodeID, body); err != nil {
		s.ops.Release(owner, appName)
		return "", err
	}
	for _, sibling := range records {
		sibling.Status = "building"
		s.publishContainer(owner, sibling)
	}
	return rec.ID, nil
}

// projectRecords returns the clicked deployment together with every compose
// service of the same project. The backend has no parent column: compose
// children are stored as "<project uuid>-<service>", so the shared id prefix
// is what links them. A single-container deployment is its own project.
func (s *Service) projectRecords(ctx context.Context, owner string, clicked *admin.ContainerRecord) ([]admin.ContainerRecord, error) {
	all, err := s.store.ListByOwner(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]admin.ContainerRecord, 0, len(all))
	for _, rec := range all {
		if sameDeploymentProject(clicked.ID, rec.ID) {
			out = append(out, rec)
		}
	}
	if len(out) == 0 {
		out = append(out, *clicked)
	}
	return out, nil
}

// deploymentProjectID strips a compose service suffix from a deployment id.
// Deployment ids are UUIDs, so the project part ends at the 36th char.
func deploymentProjectID(id string) string {
	const uuidLen = 36
	if len(id) > uuidLen && id[uuidLen] == '-' {
		return id[:uuidLen]
	}
	return id
}

func sameDeploymentProject(clickedID, candidateID string) bool {
	projectID := deploymentProjectID(clickedID)
	return candidateID == projectID || strings.HasPrefix(candidateID, projectID+"-")
}

// RemoveByID deletes the user's container: owner-scoped, one op per app.
func (s *Service) RemoveByID(ctx context.Context, owner, id string) error {
	return s.containerOp(ctx, owner, id, "remove", nil)
}

// removeExpiredByID deletes an expired deployment together with its data.
// Expiry is final, so the agent is told to drop the volumes as well; a normal
// manual remove keeps them.
func (s *Service) removeExpiredByID(ctx context.Context, owner, id string) error {
	return s.containerOp(ctx, owner, id, "remove", map[string]any{"remove_volumes": true})
}

// RemoveDeployment deletes a whole deployment: the record itself plus all
// «app-service» children (multi-service compose). Best effort.
func (s *Service) RemoveDeployment(ctx context.Context, owner, parentID string) error {
	recs, err := s.store.ListByOwner(ctx, owner)
	if err != nil {
		return err
	}
	matched := false
	for _, rec := range recs {
		if rec.ID != parentID && !strings.HasPrefix(rec.ID, parentID+"-") {
			continue
		}
		matched = true
		if err := s.RemoveByID(ctx, owner, rec.ID); err != nil {
			s.logger.Warn("deployment cleanup remove failed", zap.String("deployment", rec.ID), zap.Error(err))
		}
	}
	if !matched {
		return ErrNotFound
	}
	return nil
}

// StartByID starts the user's container (docker restart also starts stopped ones).
func (s *Service) StartByID(ctx context.Context, owner, id string) error {
	return s.containerOp(ctx, owner, id, "restart", nil)
}

// StopByID stops the user's container.
func (s *Service) StopByID(ctx context.Context, owner, id string) error {
	return s.containerOp(ctx, owner, id, "stop", nil)
}

func (s *Service) containerOp(ctx context.Context, owner, id, action string, extra map[string]any) error {
	if s.queue == nil {
		return ErrNoQueue
	}
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return ErrNotFound
	}
	// Без проверки узла задача ушла бы в очередь молчаливого воркера,
	// а замок снялся бы только по TTL — удаление «висло» бы минутами.
	if err := s.requireOnlineNode(ctx, rec.NodeID); err != nil {
		return err
	}
	if err := s.ops.Acquire(rec.OwnerLogin, rec.AppName, action); err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{
		"type":          action,
		"deployment_id": rec.ID,
		"app_id":        rec.AppID,
		"app_name":      rec.AppName,
		"owner":         rec.OwnerLogin,
	})
	if err := s.queue.Publish(ctx, "agent."+rec.NodeID, body); err != nil {
		s.ops.Release(rec.OwnerLogin, rec.AppName)
		return err
	}
	return nil
}

// GetRecord returns the container when it belongs to owner.
func (s *Service) GetRecord(ctx context.Context, owner, id string) (*admin.ContainerRecord, error) {
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return nil, ErrNotFound
	}
	return rec, nil
}

// AgentBuildLog proxies the build log stored on the worker.
func (s *Service) AgentBuildLog(ctx context.Context, nodeID, deploymentID string) ([]byte, error) {
	return s.agentGet(ctx, nodeID, "/deployments/"+deploymentID+"/build-log?tail=10000")
}

// AgentBuildLogStreamURL builds the worker WebSocket URL for live build logs.
// The agent token goes into ?token= because browsers cannot set headers.
func (s *Service) AgentBuildLogStreamURL(ctx context.Context, nodeID, deploymentID string) (string, error) {
	apiURL, err := s.agents.AgentApiURL(ctx, nodeID)
	if err != nil {
		return "", err
	}
	ws := apiURL
	switch {
	case strings.HasPrefix(ws, "https://"):
		ws = "wss://" + strings.TrimPrefix(ws, "https://")
	case strings.HasPrefix(ws, "http://"):
		ws = "ws://" + strings.TrimPrefix(ws, "http://")
	}
	return trimSlash(ws) + "/deployments/" + deploymentID + "/build-log/stream?token=" + url.QueryEscape(s.agentToken), nil
}

// AgentContainerLogStreamURL builds the worker WebSocket URL for live logs.
// app_name/owner передаются агенту, чтобы он нашёл контейнер приложения,
// даже если запись в центре ссылается на другой (неудачный) деплой.
func (s *Service) AgentContainerLogStreamURL(ctx context.Context, nodeID, deploymentID, appName, owner string) (string, error) {
	apiURL, err := s.agents.AgentApiURL(ctx, nodeID)
	if err != nil {
		return "", err
	}
	ws := apiURL
	switch {
	case strings.HasPrefix(ws, "https://"):
		ws = "wss://" + strings.TrimPrefix(ws, "https://")
	case strings.HasPrefix(ws, "http://"):
		ws = "ws://" + strings.TrimPrefix(ws, "http://")
	}
	return trimSlash(ws) + "/containers/" + url.PathEscape(deploymentID) + "/logs/stream?token=" + url.QueryEscape(s.agentToken) + "&history=0&app_name=" + url.QueryEscape(appName) + "&owner=" + url.QueryEscape(owner), nil
}

// AdminStopContainer stops a container by deployment ID (admin level, no owner check).
// Used by disk monitor and admin actions.
func (s *Service) AdminStopContainer(ctx context.Context, id string) error {
	if s.queue == nil {
		return ErrNoQueue
	}
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	body, _ := json.Marshal(map[string]string{
		"type":          "stop",
		"deployment_id": rec.ID,
		"app_id":        rec.AppID,
		"app_name":      rec.AppName,
		"owner":         rec.OwnerLogin,
	})
	if err := s.queue.Publish(ctx, "agent."+rec.NodeID, body); err != nil {
		return err
	}
	return nil
}

// DiskMonitorConfig configures the disk usage monitor.
type DiskMonitorConfig struct {
	CheckInterval time.Duration // how often to check (default 5m)
	GracePercent  float64       // extra percentage before enforcement (default 0.1 = 10%)
}

// DefaultDiskMonitorConfig returns sensible defaults.
func DefaultDiskMonitorConfig() DiskMonitorConfig {
	return DiskMonitorConfig{
		CheckInterval: 5 * time.Minute,
		GracePercent:  0.1, // 10% grace
	}
}

// StartDiskMonitor starts a background goroutine that periodically checks
// container disk usage against their DiskGB limits and stops containers
// that exceed their limit (with grace percentage).
func (s *Service) StartDiskMonitor(ctx context.Context, cfg DiskMonitorConfig) {
	if cfg.CheckInterval <= 0 {
		cfg = DefaultDiskMonitorConfig()
	}
	go func() {
		ticker := time.NewTicker(cfg.CheckInterval)
		defer ticker.Stop()
		s.logger.Info("disk monitor started", zap.Duration("interval", cfg.CheckInterval))
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("disk monitor stopped")
				return
			case <-ticker.C:
				s.checkDiskUsage(ctx, cfg.GracePercent)
			}
		}
	}()
}

// ExpiryMonitorConfig configures the expiry monitor.
type ExpiryMonitorConfig struct {
	CheckInterval time.Duration // how often to check (default 1m)
}

// DefaultExpiryMonitorConfig returns sensible defaults.
func DefaultExpiryMonitorConfig() ExpiryMonitorConfig {
	return ExpiryMonitorConfig{CheckInterval: time.Minute}
}

// StartExpiryMonitor starts a background goroutine that removes deployments
// whose expires_at has passed. Status is deliberately ignored: the user may
// have stopped the container manually, but it still has to be removed.
func (s *Service) StartExpiryMonitor(ctx context.Context, cfg ExpiryMonitorConfig) {
	if cfg.CheckInterval <= 0 {
		cfg = DefaultExpiryMonitorConfig()
	}
	go func() {
		ticker := time.NewTicker(cfg.CheckInterval)
		defer ticker.Stop()
		s.logger.Info("expiry monitor started", zap.Duration("interval", cfg.CheckInterval))
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("expiry monitor stopped")
				return
			case <-ticker.C:
				s.removeExpired(ctx)
			}
		}
	}()
}

// removeExpired removes every container whose expires_at has passed. Each
// record is removed on its own, so a deployment with diverging per-service
// expiry dates does not lose a still-valid service. The shared project folder
// is cleaned up by the agent when its last deployment is gone. A failed
// removal keeps the record in the database, so the next tick retries it.
func (s *Service) removeExpired(ctx context.Context) {
	recs, err := s.store.ListContainers(ctx)
	if err != nil {
		s.logger.Warn("expiry monitor: failed to list containers", zap.Error(err))
		return
	}
	now := time.Now()
	for _, rec := range recs {
		if rec.ExpiresAt == nil || rec.ExpiresAt.After(now) {
			continue
		}
		s.logger.Info("expiry monitor: removing expired container",
			zap.String("deployment", rec.ID),
			zap.String("app", rec.AppName),
			zap.String("owner", rec.OwnerLogin))
		if err := s.RemoveByID(ctx, rec.OwnerLogin, rec.ID); err != nil {
			s.logger.Warn("expiry monitor: remove failed",
				zap.String("deployment", rec.ID), zap.Error(err))
		}
	}
}

// RequestContainerUpdate creates a deployment request for updating container parameters.
// This is called by users when they want to change container settings.
// The request needs admin approval before the container is recreated with new params.
// Returns the request ID.
func (s *Service) RequestContainerUpdate(ctx context.Context, owner, deploymentID string, params map[string]any) (string, error) {
	rec, err := s.store.GetContainer(ctx, deploymentID)
	if err != nil {
		return "", ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return "", ErrNotFound
	}
	// Fundamental parameters cannot be changed by an update request either:
	// the running container keeps its address and image.
	if raw, ok := params["ports"]; ok {
		var requested []recreatePort
		if data, err := json.Marshal(raw); err == nil {
			if err := json.Unmarshal(data, &requested); err == nil && !samePortMappings(rec.Ports, requested) {
				return "", errors.New("порты запущенного контейнера изменить нельзя")
			}
		}
	}
	if image, ok := params["image"].(string); ok && image != "" && rec.Image != "" && image != rec.Image {
		return "", errors.New("образ запущенного контейнера изменить нельзя — используйте обновление")
	}

	// Acquire operation lock to prevent concurrent operations
	if err := s.ops.Acquire(rec.OwnerLogin, rec.AppName, "request_update"); err != nil {
		return "", err
	}
	defer s.ops.Release(rec.OwnerLogin, rec.AppName)

	// Create the deployment request directly in the database
	// Use the admin repository's pool to do a direct insert
	requestID := uuid.NewString()

	// Ports and image are immutable, so the request always carries the current
	// values (and the deploy-shaped JSON the request repository expects).
	portsJSON, _ := json.Marshal(rec.Ports)

	// Build volumes - use direct marshal since rec.Volumes is now []VolumeSpec
	volumesVal := getOrDefault(params, "volumes", rec.Volumes)
	volumesJSON, _ := json.Marshal(volumesVal)

	// Build env - use direct marshal since rec.Env is now map[string]string
	envVal := getOrDefault(params, "env", rec.Env)
	envJSON, _ := json.Marshal(envVal)

	// Build services JSON (empty for updates)
	servicesJSON := "[]"

	// Get restart_policy (default to "unless-stopped" if not specified)
	restartPolicy := getOrDefaultString(params, "restart_policy", "unless-stopped")

	// Insert into launch_requests table
	_, err = s.store.(*admin.Repository).Pool().Exec(ctx, `
		INSERT INTO launch_requests
		(id, owner_login, app_name, repo_url, image, container_name, node_id, ports, volumes, env, cpu_cores, memory_gb, disk_gb, duration_hours, branch, services, build_mode, price, status, type, existing_deployment_id, restart_policy, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,now(),now())`,
		requestID, rec.OwnerLogin, rec.AppName, "", // repo_url empty for updates
		getOrDefaultString(params, "image", rec.Image),
		rec.AppName, // container_name = app_name
		rec.NodeID,
		string(portsJSON),
		string(volumesJSON),
		string(envJSON),
		getOrDefaultFloat64(params, "cpu_cores", rec.CPUCores),
		getOrDefaultFloat64(params, "memory_gb", rec.MemoryGB),
		getOrDefaultFloat64(params, "disk_gb", rec.DiskGB),
		0,  // duration_hours not applicable for updates
		"", // branch empty for updates
		servicesJSON,
		"", // build_mode empty for updates
		0,  // price = 0 for updates
		"pending",
		"update",
		deploymentID,
		restartPolicy,
	)
	if err != nil {
		return "", err
	}

	return requestID, nil
}

// Helper methods to convert to JSON
func (s *Service) convertPortsToJSON(ports any) []map[string]any {
	if portArr, ok := ports.([]admin.PortMapping); ok {
		result := make([]map[string]any, 0, len(portArr))
		for _, p := range portArr {
			result = append(result, map[string]any{
				"host_port":      p.HostPort,
				"container_port": p.ContainerPort,
			})
		}
		return result
	}
	if portArr, ok := ports.([]any); ok {
		result := make([]map[string]any, 0, len(portArr))
		for _, item := range portArr {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result
	}
	return []map[string]any{}
}

func (s *Service) convertVolumesToJSON(volumes any) []map[string]any {
	if volStr, ok := volumes.(string); ok {
		var result []map[string]any
		_ = json.Unmarshal([]byte(volStr), &result)
		return result
	}
	if volArr, ok := volumes.([]any); ok {
		result := make([]map[string]any, 0, len(volArr))
		for _, item := range volArr {
			if m, ok := item.(map[string]any); ok {
				result = append(result, m)
			}
		}
		return result
	}
	return []map[string]any{}
}

func getOrDefaultString(params map[string]any, key, defaultVal string) string {
	if val, ok := params[key]; ok && val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getOrDefaultFloat64(params map[string]any, key string, defaultVal float64) float64 {
	if val, ok := params[key]; ok && val != nil {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return defaultVal
}

// getOrDefault returns the value from params if present, otherwise returns defaultVal
func getOrDefault(params map[string]any, key string, defaultVal any) any {
	if val, ok := params[key]; ok && val != nil {
		return val
	}
	return defaultVal
}

// checkDiskUsage queries all nodes for container sizes and stops containers
// exceeding their DiskGB limit (with grace).
func (s *Service) checkDiskUsage(ctx context.Context, gracePercent float64) {
	// Get all running containers from the database
	containers, err := s.store.ListContainers(ctx)
	if err != nil {
		s.logger.Warn("disk monitor: failed to list containers", zap.Error(err))
		return
	}
	if len(containers) == 0 {
		return
	}

	// Group containers by node
	byNode := make(map[string][]admin.ContainerRecord)
	for _, c := range containers {
		if c.Status == "running" && c.DiskGB > 0 && c.NodeID != "" {
			byNode[c.NodeID] = append(byNode[c.NodeID], c)
		}
	}

	// Check each node
	for nodeID, nodeContainers := range byNode {
		sizes, err := s.GetContainerSizes(ctx, nodeID)
		if err != nil {
			s.logger.Warn("disk monitor: failed to get container sizes", zap.String("node", nodeID), zap.Error(err))
			continue
		}

		// Build map of deployment ID -> size (агент уже сопоставил
		// docker container ID с deployment ID).
		sizeMap := make(map[string]int64)
		for _, sz := range sizes {
			if sz.DeploymentID != "" && sz.DeploymentID != "unknown" {
				sizeMap[sz.DeploymentID] = sz.SizeBytes
			}
		}

		// Check each container
		for _, rec := range nodeContainers {
			sizeBytes := sizeMap[rec.ID]
			if sizeBytes == 0 {
				continue
			}

			limitBytes := int64(rec.DiskGB * 1024 * 1024 * 1024)
			graceLimit := int64(float64(limitBytes) * (1 + gracePercent))

			if sizeBytes > graceLimit {
				s.logger.Warn("disk monitor: container exceeds disk limit, stopping",
					zap.String("deployment", rec.ID),
					zap.String("app", rec.AppName),
					zap.Float64("limit_gb", rec.DiskGB),
					zap.Int64("used_bytes", sizeBytes),
					zap.Int64("limit_bytes", limitBytes),
				)

				if err := s.AdminStopContainer(ctx, rec.ID); err != nil {
					s.logger.Error("disk monitor: failed to stop container",
						zap.String("deployment", rec.ID), zap.Error(err))
				} else {
					_ = s.store.SetContainerStatus(ctx, rec.ID, "disk_exceeded")
				}
			}
		}
	}
}
