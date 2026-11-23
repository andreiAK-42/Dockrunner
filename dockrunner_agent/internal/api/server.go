package api

import (
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger"

	"dockrunner_agent/internal/buildlog"
	"dockrunner_agent/internal/config"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/logs"

	_ "dockrunner_agent/docs"
)

type HealthResponse struct {
	Status  string `json:"status" example:"ok"`
	AgentID string `json:"agent_id" example:"worker-01"`
	Version string `json:"version" example:"0.1.0"`
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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /containers", s.checkToken(s.handleContainers))
	mux.HandleFunc("GET /containers/sizes", s.checkToken(s.handleContainerSizes))
	mux.HandleFunc("GET /containers/{id}/logs", s.checkToken(s.handleContainerLogs))
	mux.HandleFunc("GET /containers/{id}/spec", s.checkToken(s.handleContainerSpec))
	mux.HandleFunc("POST /containers/{id}/recreate", s.checkToken(s.handleContainerRecreate))
	mux.HandleFunc("GET /containers/{id}/stats", s.checkToken(s.handleContainerStats))
	mux.HandleFunc("GET /containers/{id}/logs/stream", s.checkToken(s.handleContainerLogStream))
	mux.HandleFunc("GET /deployments/{id}/build-log", s.checkToken(s.handleBuildLog))
	mux.HandleFunc("GET /deployments/{id}/build-log/stream", s.checkToken(s.handleBuildLogStream))
	mux.Handle("GET /swagger/", httpSwagger.Handler())
	return withCORS(mux)
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
