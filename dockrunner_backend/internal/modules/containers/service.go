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
	"dockrunner_backend/pkg/queue"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrNoQueue     = errors.New("queue unavailable")
	ErrUnknownNode = errors.New("узел не присылал heartbeat — проверьте, что агент запущен")
	ErrNodeOffline = errors.New("узел не в сети — нет свежего heartbeat")
)

type Check struct {
	ID                 string   `json:"id"`
	AppName            string   `json:"app_name"`
	RepoURL            string   `json:"repo_url"`
	NodeID             string   `json:"node_id"`
	Status             string   `json:"status"`
	DetectedDockerfile bool     `json:"detected_dockerfile"`
	ComposeFile        string   `json:"compose_file"`
	Services           []string `json:"services"`
	Error              string   `json:"error"`
	owner              string
	expires            time.Time
}

type ContainerStore interface {
	GetContainer(ctx context.Context, id string) (*admin.ContainerRecord, error)
	FindLatestByApp(ctx context.Context, owner, appName string) (*admin.ContainerRecord, error)
	ListByOwner(ctx context.Context, owner string) ([]admin.ContainerRecord, error)
	ListByNode(ctx context.Context, nodeID string) ([]admin.ContainerRecord, error)
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
}

func NewService(store ContainerStore, agents *agents.Repository, vols *volumes.Service, agentToken string, queue *queue.Client, logger *zap.Logger, ops *oplock.Tracker, ports PortsPolicy) *Service {
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
	}
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

func (s *Service) ContainerSpec(ctx context.Context, owner, id string) ([]byte, error) {
	rec, err := s.store.GetContainer(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	if rec.OwnerLogin != "" && rec.OwnerLogin != owner {
		return nil, ErrNotFound
	}
	return s.agentGet(ctx, rec.NodeID, "/containers/"+id+"/spec")
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
	body, err := s.agentGet(ctx, rec.NodeID, "/containers/"+id+"/stats")
	if err != nil {
		return nil, err
	}
	var stats ContainerStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
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
	body, err := s.agentGet(ctx, rec.NodeID, fmt.Sprintf("/containers/%s/logs?tail=%d", id, tail))
	if err != nil {
		return "", err
	}
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	return payload.Data, nil
}

func trimSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func (s *Service) StartCheck(ctx context.Context, owner, appName, repoURL, nodeID string) (*Check, error) {
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
	})
	if err := s.queue.Publish(ctx, "agent."+nodeID, body); err != nil {
		s.mu.Lock()
		delete(s.checks, check.ID)
		s.mu.Unlock()
		s.ops.Release(owner, appName)
		return nil, err
	}
	s.logger.Info("prepare task published", zap.String("node", nodeID), zap.String("check", check.ID))
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

func (s *Service) ResolveCheck(deploymentID string, ok bool, dockerfile bool, composeFile string, services []string, errMsg string) {
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
	} else {
		check.Status = "failed"
		check.Error = errMsg
	}
	owner, appName := check.owner, check.AppName
	s.mu.Unlock()
	s.ops.Release(owner, appName)
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
	// Services — сервисы compose (пусто = один контейнер).
	// Каждый сервис станет записью «приложение-сервис» с теми же лимитами.
	Services []string `json:"services"`
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
		}
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
	if err := s.store.SetContainerStatus(ctx, rec.ID, "building"); err != nil {
		s.ops.Release(owner, appName)
		return "", err
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
	return rec.ID, nil
}

// RemoveByID deletes the user's container: owner-scoped, one op per app.
func (s *Service) RemoveByID(ctx context.Context, owner, id string) error {
	return s.containerOp(ctx, owner, id, "remove")
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
	return s.containerOp(ctx, owner, id, "restart")
}

// StopByID stops the user's container.
func (s *Service) StopByID(ctx context.Context, owner, id string) error {
	return s.containerOp(ctx, owner, id, "stop")
}

func (s *Service) containerOp(ctx context.Context, owner, id, action string) error {
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
