package containers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	"dockrunner_backend/internal/middleware"
	"dockrunner_backend/internal/modules/agents"
	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/pkg/response"

	"github.com/coder/websocket"
)

type Handler struct {
	service   *Service
	logger    *zap.Logger
	jwtSecret string
}

func NewHandler(service *Service, logger *zap.Logger, jwtSecret string) *Handler {
	return &Handler{service: service, logger: logger, jwtSecret: jwtSecret}
}

type CheckRequest struct {
	AppName string `json:"app_name" binding:"required" example:"my-awesome-app"`
	RepoURL string `json:"repo_url" binding:"required,url" example:"https://github.com/org/repo"`
	NodeID  string `json:"node_id" binding:"required" example:"worker-01"`
	Branch  string `json:"branch" binding:"required" example:"main"`
}

func parseTail(raw string) int {
	if raw == "" {
		return 200
	}
	tail, err := strconv.Atoi(raw)
	if err != nil || tail <= 0 {
		return 200
	}
	return tail
}

// Mine godoc
// @Summary Мои контейнеры
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]admin.ContainerRecord}
// @Router /api/v1/containers/mine [get]
func (h *Handler) Mine(c *gin.Context) {
	containers, err := h.service.Mine(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("mine failed", zap.Error(err))
		response.Internal(c, "failed to list containers")
		return
	}
	response.OK(c, containers)
}

// ContainerLogs godoc
// @Summary Хвост логов контейнера через воркер
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID контейнера"
// @Param tail query int false "Число строк" default(200)
// @Success 200 {object} response.Success{data=string}
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/{id}/logs [get]
func (h *Handler) ContainerLogs(c *gin.Context) {
	tail := parseTail(c.Query("tail"))
	text, err := h.service.ContainerLogs(c.Request.Context(), c.GetString("login"), c.Param("id"), tail)
	if err != nil {
		h.logger.Warn("container logs failed", zap.Error(err))
		response.NotFound(c, "logs unavailable")
		return
	}
	response.OK(c, text)
}

// ContainerSpec godoc
// @Summary Полные параметры контейнера с воркера
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID контейнера"
// @Success 200 {object} response.Success
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/{id}/spec [get]
func (h *Handler) ContainerSpec(c *gin.Context) {
	spec, err := h.service.ContainerSpec(c.Request.Context(), c.GetString("login"), c.Param("id"))
	if err != nil {
		h.logger.Warn("container spec failed", zap.Error(err))
		response.NotFound(c, "spec unavailable")
		return
	}
	c.Data(http.StatusOK, "application/json", append(append([]byte(`{"data":`), spec...), []byte(`}`)...))
}

// RecreateContainer applies edited Docker launch parameters by recreating the container.
// This is an admin endpoint - users should use RequestUpdate instead.
func (h *Handler) RecreateContainer(c *gin.Context) {
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	body, err := h.service.RecreateContainer(c.Request.Context(), c.GetString("login"), c.Param("id"), req)
	if err != nil {
		h.logger.Warn("container recreate failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "container not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.Data(http.StatusOK, "application/json", body)
}

// RequestUpdate creates a deployment request for updating container parameters.
// Users call this instead of directly recreating - the request needs admin approval.
func (h *Handler) RequestUpdate(c *gin.Context) {
	var req map[string]any
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	requestID, err := h.service.RequestContainerUpdate(c.Request.Context(), c.GetString("login"), c.Param("id"), req)
	if err != nil {
		h.logger.Warn("container update request failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "container not found")
			return
		}
		if errors.Is(err, ErrNoQueue) {
			response.Internal(c, "queue unavailable")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "update request created, waiting for admin approval", "request_id": requestID})
}

// ContainerStats godoc
// @Summary Живая статистика контейнера (CPU %, RAM МБ) через воркер
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID контейнера"
// @Success 200 {object} response.Success{data=containers.ContainerStats}
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/{id}/stats [get]
func (h *Handler) ContainerStats(c *gin.Context) {
	stats, err := h.service.ContainerStats(c.Request.Context(), c.GetString("login"), c.Param("id"))
	if err != nil {
		h.logger.Warn("container stats failed", zap.Error(err))
		response.NotFound(c, "stats unavailable")
		return
	}
	response.OK(c, stats)
}

// StartCheck godoc
// @Summary Проверка репозитория на воркере
// @Tags containers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body containers.CheckRequest true "Приложение, репозиторий, узел, ветка"
// @Success 202 {object} response.Success{data=containers.Check}
// @Failure 400 {object} response.Error
// @Router /api/v1/containers/check [post]
func (h *Handler) StartCheck(c *gin.Context) {
	var req CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	check, err := h.service.StartCheck(c.Request.Context(), c.GetString("login"), req.AppName, req.RepoURL, req.NodeID, req.Branch)
	if err != nil {
		h.logger.Error("check failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		if err == ErrUnknownNode || err == ErrNoQueue || err == ErrNodeOffline {
			response.BadRequest(c, err.Error())
			return
		}
		response.Internal(c, "failed to start repository check")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": check})
}

// GetCheck godoc
// @Summary Статус проверки для опроса
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID проверки"
// @Success 200 {object} response.Success{data=containers.Check}
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/check/{id} [get]
func (h *Handler) GetCheck(c *gin.Context) {
	check, err := h.service.GetCheck(c.Param("id"))
	if err != nil {
		response.NotFound(c, "check not found")
		return
	}
	response.OK(c, check)
}

// Default container resource limits
const (
	DefaultCPUCores  = 0.25
	DefaultMemoryGB  = 0.5  // 512 MB
	DefaultDiskGB    = 1.0
	MinCPUCores      = 0.01 // prevent zero/negative
	MinMemoryGB      = 0.01 // 10 MB minimum
	MinDiskGB        = 0.1  // 100 MB minimum
)

type RunRequest struct {
	AppName       string            `json:"app_name" binding:"required" example:"my-awesome-app"`
	RepoURL       string            `json:"repo_url" example:"https://github.com/org/repo"`
	Image         string            `json:"image" example:""`
	ContainerName string            `json:"container_name" example:"web-01"`
	NodeID        string            `json:"node_id" binding:"required" example:"worker-01"`
	Ports         []PortSpec        `json:"ports"`
	Volumes       []VolumeSpec      `json:"volumes"`
	Env           map[string]string `json:"env"`
	CPUCores      float64           `json:"cpu_cores" example:"0.25"`
	MemoryGB      float64           `json:"memory_gb" example:"0.5"`
	DiskGB        float64           `json:"disk_gb" example:"1"`
	DurationHours float64           `json:"duration_hours" example:"10"`
	Services      []string          `json:"services" example:"front,back"`
	BuildMode     string            `json:"build_mode" example:"compose"`
}

func applyDefaultsAndValidate(req *RunRequest) error {
	// Apply defaults
	if req.CPUCores <= 0 {
		req.CPUCores = DefaultCPUCores
	}
	if req.MemoryGB <= 0 {
		req.MemoryGB = DefaultMemoryGB
	}
	if req.DiskGB <= 0 {
		req.DiskGB = DefaultDiskGB
	}
	// Validate minimums
	if req.CPUCores < MinCPUCores {
		return errors.New("cpu_cores must be at least 0.01")
	}
	if req.MemoryGB < MinMemoryGB {
		return errors.New("memory_gb must be at least 0.01 (10 MB)")
	}
	if req.DiskGB < MinDiskGB {
		return errors.New("disk_gb must be at least 0.1 (100 MB)")
	}
	return nil
}

// Run godoc
// @Summary Запустить контейнер в стиле docker run
// @Tags containers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body containers.RunRequest true "Имя, образ или репозиторий, порты, пути, env, ресурсы"
// @Success 202 {object} response.Success
// @Failure 400 {object} response.Error
// @Router /api/v1/containers/run [post]
func (h *Handler) Run(c *gin.Context) {
	var req RunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := applyDefaultsAndValidate(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	id, err := h.service.Run(c.Request.Context(), c.GetString("login"), RunInput{
		AppName:       req.AppName,
		RepoURL:       req.RepoURL,
		Image:         req.Image,
		ContainerName: req.ContainerName,
		NodeID:        req.NodeID,
		Ports:         req.Ports,
		Volumes:       req.Volumes,
		Env:           req.Env,
		CPUCores:      req.CPUCores,
		MemoryGB:      req.MemoryGB,
		DiskGB:        req.DiskGB,
		DurationHours: req.DurationHours,
		Services:      req.Services,
		BuildMode:     req.BuildMode,
	})
	if err != nil {
		h.logger.Warn("run failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"deployment_id": id}})
}

type RefreshRequest struct {
	AppName string `json:"app_name" binding:"required" example:"my-awesome-app"`
}

// Refresh godoc
// @Summary Обновить контейнер: свежий pull, пересборка, старт
// @Tags containers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body containers.RefreshRequest true "Имя приложения"
// @Success 202 {object} response.Success
// @Failure 400 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	id, err := h.service.RefreshByApp(c.Request.Context(), c.GetString("login"), req.AppName)
	if err != nil {
		h.logger.Warn("refresh failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"deployment_id": id}})
}

// Remove godoc
// @Summary Удалить своё приложение (контейнер на воркере)
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID деплоя"
// @Success 202 {object} response.Success
// @Failure 404 {object} response.Error
// @Failure 409 {object} response.Error
// @Router /api/v1/containers/{id} [delete]
func (h *Handler) Remove(c *gin.Context) {
	if err := h.service.RemoveByID(c.Request.Context(), c.GetString("login"), c.Param("id")); err != nil {
		if err == ErrNotFound {
			response.NotFound(c, "application not found")
			return
		}
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		h.logger.Warn("remove failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"message": "remove started"}})
}

// Start godoc
// @Summary Запустить свой контейнер (docker restart поднимает и остановленные)
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID деплоя"
// @Success 202 {object} response.Success
// @Failure 404 {object} response.Error
// @Failure 409 {object} response.Error
// @Router /api/v1/containers/{id}/start [post]
func (h *Handler) Start(c *gin.Context) {
	h.containerOp(c, "start")
}

// Stop godoc
// @Summary Остановить свой контейнер
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID деплоя"
// @Success 202 {object} response.Success
// @Failure 404 {object} response.Error
// @Failure 409 {object} response.Error
// @Router /api/v1/containers/{id}/stop [post]
func (h *Handler) Stop(c *gin.Context) {
	h.containerOp(c, "stop")
}

func (h *Handler) containerOp(c *gin.Context, action string) {
	var err error
	switch action {
	case "start":
		err = h.service.StartByID(c.Request.Context(), c.GetString("login"), c.Param("id"))
	case "stop":
		err = h.service.StopByID(c.Request.Context(), c.GetString("login"), c.Param("id"))
	default:
		response.BadRequest(c, "unknown action")
		return
	}
	if err != nil {
		if err == ErrNotFound {
			response.NotFound(c, "application not found")
			return
		}
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		h.logger.Warn("container op failed", zap.String("action", action), zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"message": action + " started"}})
}

// BuildLog godoc
// @Summary Лог сборки деплоя с воркера (шаги Dockerfile, скачивание слоёв)
// @Tags containers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID деплоя"
// @Success 200 {object} response.Success{data=string}
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/{id}/build-log [get]
func (h *Handler) BuildLog(c *gin.Context) {
	rec, err := h.service.GetRecord(c.Request.Context(), c.GetString("login"), c.Param("id"))
	if err != nil {
		response.NotFound(c, "application not found")
		return
	}
	body, err := h.service.AgentBuildLog(c.Request.Context(), rec.NodeID, rec.ID)
	if err != nil {
		h.logger.Warn("build log failed", zap.Error(err))
		if errors.Is(err, agents.ErrNoAPIURL) {
			response.NotFound(c, "воркер не сообщил свой адрес API — задайте AGENT_PUBLIC_URL у агента")
			return
		}
		response.NotFound(c, "build log unavailable")
		return
	}
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		response.NotFound(c, "build log unavailable")
		return
	}
	response.OK(c, payload.Data)
}

// BuildLogStream godoc
// @Summary Живой лог сборки по WebSocket (прокси на воркер)
// @Description Токен — в ?token=, браузер не ставит заголовки. Сначала история, затем живые строки.
// @Tags containers
// @Produce json
// @Param id path string true "ID деплоя"
// @Param token query string true "JWT пользователя"
// @Success 101 "Переключение на WebSocket"
// @Failure 401 {object} response.Error
// @Failure 404 {object} response.Error
// @Router /api/v1/containers/{id}/build-log/stream [get]
func (h *Handler) BuildLogStream(c *gin.Context) {
	claims, ok := h.queryToken(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}
	rec, err := h.service.GetRecord(c.Request.Context(), claims.Login, c.Param("id"))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	wsURL, err := h.service.AgentBuildLogStreamURL(c.Request.Context(), rec.NodeID, rec.ID)
	if err != nil {
		msg := "build log unavailable"
		if errors.Is(err, agents.ErrNoAPIURL) {
			msg = "воркер не сообщил свой адрес API — задайте AGENT_PUBLIC_URL у агента"
		}
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": msg})
		return
	}
	clientConn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer func() { _ = clientConn.Close(websocket.StatusNormalClosure, "") }()

	dialCtx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	agentConn, _, err := websocket.Dial(dialCtx, wsURL, nil)
	if err != nil {
		h.logger.Warn("build log stream dial failed", zap.Error(err))
		_ = clientConn.Close(websocket.StatusInternalError, "agent unreachable")
		return
	}
	defer func() { _ = agentConn.Close(websocket.StatusNormalClosure, "") }()

	ctx, stop := context.WithCancel(c.Request.Context())
	defer stop()
	go func() {
		defer stop()
		for {
			if _, _, err := clientConn.Read(ctx); err != nil {
				return
			}
		}
	}()
	for {
		typ, msg, err := agentConn.Read(ctx)
		if err != nil {
			return
		}
		if err := clientConn.Write(ctx, typ, msg); err != nil {
			return
		}
	}
}

func (h *Handler) queryToken(c *gin.Context) (*middleware.Claims, bool) {
	raw := c.Query("token")
	if raw == "" {
		return nil, false
	}
	claims := &middleware.Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return nil, false
	}
	return claims, true
}
