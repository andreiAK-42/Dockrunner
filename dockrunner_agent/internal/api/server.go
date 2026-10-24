package api

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/coder/websocket"
	httpSwagger "github.com/swaggo/http-swagger"

	"dockrunner_agent/internal/buildlog"
	"dockrunner_agent/internal/config"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/logs"
	"dockrunner_agent/internal/store"

	_ "dockrunner_agent/docs"
)

type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	AgentID string `json:"agent_id" example:"worker-01"`
	Version string `json:"version" example:"0.1.0"`
}

type ContainerItem struct {
	ID        string `json:"id" example:"dep-1048"`
	AppName   string `json:"app_name" example:"frontend-web"`
	Image     string `json:"image" example:"nginx:1.27"`
	Container string `json:"container" example:"a1b2c3d4e5f6"`
	Stored    string `json:"stored_status" example:"running"`
	Live      string `json:"live_status" example:"running"`
}

type ContainersResponse struct {
	Data []ContainerItem `json:"data"`
}

type LogsResponse struct {
	Data string `json:"data"`
}

type StatsResponse struct {
	CPUPercent float64 `json:"cpu_percent" example:"12.5"`
	MemMB      float64 `json:"mem_mb" example:"256.4"`
}

type ErrorResponse struct {
	Error string `json:"error" example:"deployment not found"`
}

type Server struct {
	cfg      *config.Config
	db       *sql.DB
	dock     *docker.Client
	streamer *logs.Streamer
	builds   *buildlog.Store
	logger   func(msg string, args ...any)
}

func New(cfg *config.Config, db *sql.DB, dock *docker.Client, streamer *logs.Streamer, builds *buildlog.Store, log func(string, ...any)) *Server {
	if builds == nil {
		builds = buildlog.New(db)
	}
	return &Server{cfg: cfg, db: db, dock: dock, streamer: streamer, builds: builds, logger: log}
}

func (s *Server) checkToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token == "" {
			next(w, r)
			return
		}
		auth := bearerFromRequest(r)
		want := "Bearer " + s.cfg.Token
		if subtle.ConstantTimeCompare([]byte(auth), []byte(want)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// bearerFromRequest достаёт токен из заголовка или ?token= (для браузера).
func bearerFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth != "" {
		return auth
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return "Bearer " + token
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleHealth godoc
// @Summary Проверка живости агента
// @Tags agent
// @Produce json
// @Success 200 {object} api.HealthResponse
// @Router /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:  "ok",
		AgentID: s.cfg.AgentName,
		Version: config.Version,
	})
}

// handleContainers godoc
// @Summary Контейнеры воркера (из SQLite + живой статус)
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Success 200 {object} api.ContainersResponse
// @Failure 401 {object} api.ErrorResponse
// @Router /containers [get]
func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	deployments, err := store.ListDeployments(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]ContainerItem, 0, len(deployments))
	for _, d := range deployments {
		live := s.liveStatus(r.Context(), d.Container, d.Status)
		out = append(out, ContainerItem{ID: d.ID, AppName: d.AppName, Image: d.Image, Container: d.Container, Stored: d.Status, Live: live})
	}
	writeJSON(w, http.StatusOK, ContainersResponse{Data: out})
}

func (s *Server) findContainer(r *http.Request) (*store.Deployment, bool) {
	dep, err := store.GetDeployment(s.db, r.PathValue("id"))
	if err != nil {
		return nil, false
	}
	return dep, true
}

// liveStatus — живой статус контейнера; при ошибке docker — записанный,
// пропавший контейнер помечается "missing".
func (s *Server) liveStatus(ctx context.Context, containerID, stored string) string {
	live, err := s.dock.ContainerStatus(ctx, containerID)
	if err == nil {
		return live
	}
	if docker.IsMissing(err) {
		return "missing"
	}
	return stored
}

// queryTail разбирает ?tail=, по умолчанию 200.
func queryTail(r *http.Request) int {
	raw := r.URL.Query().Get("tail")
	if raw == "" {
		return 200
	}
	tail, err := strconv.Atoi(raw)
	if err != nil || tail <= 0 {
		return 200
	}
	return tail
}

type SpecPort struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

type SpecVolume struct {
	Name          string `json:"name"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
}

type SpecResponse struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Status  string            `json:"status"`
	Ports   []SpecPort        `json:"ports"`
	Volumes []SpecVolume      `json:"volumes"`
	Env     map[string]string `json:"env"`
}

// handleContainerSpec godoc
// @Summary Полные параметры контейнера из SQLite
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Success 200 {object} api.SpecResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/spec [get]
func (s *Server) handleContainerSpec(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	status := s.liveStatus(r.Context(), dep.Container, dep.Status)

	var storedPorts []struct {
		Host          int `json:"host"`
		Container     int `json:"container"`
		HostPort      int `json:"host_port"`
		ContainerPort int `json:"container_port"`
	}
	_ = json.Unmarshal([]byte(dep.Ports), &storedPorts)
	specPorts := make([]SpecPort, 0, len(storedPorts))
	for _, p := range storedPorts {
		host, containerPort := p.Host, p.Container
		if host == 0 {
			host = p.HostPort
		}
		if containerPort == 0 {
			containerPort = p.ContainerPort
		}
		if host > 0 && containerPort > 0 {
			specPorts = append(specPorts, SpecPort{Host: host, Container: containerPort})
		}
	}
	var volumes []struct {
		Name      string `json:"name"`
		HostPath  string `json:"host_path"`
		Container string `json:"container_path"`
		Path      string `json:"path"`
	}
	_ = json.Unmarshal([]byte(dep.Volumes), &volumes)
	specVolumes := make([]SpecVolume, 0, len(volumes))
	for _, v := range volumes {
		containerPath := v.Container
		if containerPath == "" {
			containerPath = v.Path
		}
		specVolumes = append(specVolumes, SpecVolume{Name: v.Name, HostPath: v.HostPath, ContainerPath: containerPath})
	}
	env := map[string]string{}
	_ = json.Unmarshal([]byte(dep.Env), &env)
	if env == nil {
		env = map[string]string{}
	}
	writeJSON(w, http.StatusOK, SpecResponse{
		ID: dep.ID, Name: dep.Name, Image: dep.Image, Status: status,
		Ports: specPorts, Volumes: specVolumes, Env: env,
	})
}

// handleContainerStats godoc
// @Summary Живая статистика контейнера (CPU %, RAM МБ)
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Success 200 {object} api.StatsResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/stats [get]
func (s *Server) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	stats, err := s.dock.Stats(r.Context(), dep.Container)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "stats unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, StatsResponse{CPUPercent: stats.CPUPercent, MemMB: stats.MemMB})
}

// handleContainerLogs godoc
// @Summary Хвост логов контейнера
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Param tail query int false "Число строк" default(200)
// @Success 200 {object} api.LogsResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/logs [get]
func (s *Server) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	tail := queryTail(r)
	// Docker logs are the source of truth. The in-memory streamer only retains
	// a small rolling buffer and must not replace the requested historical tail.
	text, err := s.dock.LogsTail(r.Context(), dep.Container, strconv.Itoa(tail))
	if err == nil {
		writeJSON(w, http.StatusOK, LogsResponse{Data: text})
		return
	}
	// A running stream may still have useful recent output if Docker's log
	// driver does not support reading historical logs.
	history := s.streamer.History(dep.Container, tail)
	if len(history) > 0 {
		lines := make([]string, 0, len(history))
		for _, raw := range history {
			var entry logs.Entry
			if json.Unmarshal([]byte(raw), &entry) == nil {
				lines = append(lines, entry.Line)
			}
		}
		writeJSON(w, http.StatusOK, LogsResponse{Data: strings.Join(lines, "\n")})
		return
	}
	writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
}

// handleBuildLog godoc
// @Summary Лог сборки деплоя (шаги Dockerfile, скачивание слоёв)
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Param tail query int false "Число строк" default(200)
// @Success 200 {object} api.LogsResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /deployments/{id}/build-log [get]
func (s *Server) handleBuildLog(w http.ResponseWriter, r *http.Request) {
	tail := queryTail(r)
	text, ok := s.builds.Tail(r.PathValue("id"), tail)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no build log"})
		return
	}
	writeJSON(w, http.StatusOK, LogsResponse{Data: text})
}

// handleBuildLogStream godoc
// @Summary Живой лог сборки по WebSocket
// @Description Сначала отдаёт историю (200 строк), затем живые строки текстом. Токен — в ?token=, заголовок в браузере не поставить.
// @Tags agent
// @Param id path string true "ID деплоя"
// @Param token query string false "Токен агента"
// @Success 101 "Переключение на WebSocket"
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /deployments/{id}/build-log/stream [get]
func (s *Server) handleBuildLogStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var history []string
	if text, ok := s.builds.Tail(id, 10000); ok {
		history = strings.Split(text, "\n")
	}

	serveStream(w, r, history, func() (<-chan string, func()) {
		return s.builds.Subscribe(id)
	})
}

// handleContainerLogStream godoc
// @Summary Живые логи по WebSocket
// @Description Сначала отдаёт историю (200 строк), затем живые сообщения {stream, line, time}. Токен — в ?token=, заголовок в браузере не поставить.
// @Tags agent
// @Param id path string true "ID деплоя"
// @Param token query string false "Токен агента"
// @Success 101 "Переключение на WebSocket"
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/logs/stream [get]
func (s *Server) handleContainerLogStream(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}

	serveStream(w, r, s.streamer.History(dep.Container, 200), func() (<-chan string, func()) {
		return s.streamer.Subscribe(dep.Container)
	})
}

// serveStream отдаёт историю построчно, затем живые сообщения, пока клиент на связи.
func serveStream(w http.ResponseWriter, r *http.Request, history []string, subscribe func() (<-chan string, func())) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	ctx := r.Context()

	for _, line := range history {
		if err := conn.Write(ctx, websocket.MessageText, []byte(line)); err != nil {
			return
		}
	}

	ch, unsubscribe := subscribe()
	defer unsubscribe()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
				return
			}
		}
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /containers", s.checkToken(s.handleContainers))
	mux.HandleFunc("GET /containers/{id}/logs", s.checkToken(s.handleContainerLogs))
	mux.HandleFunc("GET /containers/{id}/spec", s.checkToken(s.handleContainerSpec))
	mux.HandleFunc("GET /containers/{id}/stats", s.checkToken(s.handleContainerStats))
	mux.HandleFunc("GET /containers/{id}/logs/stream", s.checkToken(s.handleContainerLogStream))
	mux.HandleFunc("GET /deployments/{id}/build-log", s.checkToken(s.handleBuildLog))
	mux.HandleFunc("GET /deployments/{id}/build-log/stream", s.checkToken(s.handleBuildLogStream))
	mux.Handle("GET /swagger/", httpSwagger.Handler())
	return withCORS(mux)
}

// withCORS allows browser dashboards (different origin) to call the agent
// API directly, e.g. ping checks against /health.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
