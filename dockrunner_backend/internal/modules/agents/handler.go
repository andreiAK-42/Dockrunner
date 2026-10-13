package agents

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/response"
)

type Handler struct {
	service *Service
	logger  *zap.Logger
}

func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// Heartbeat godoc
// @Summary Heartbeat воркера
// @Tags agents
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer <agent_token>"
// @Param request body agents.HeartbeatInput true "Метрики воркера"
// @Success 200 {object} response.Success
// @Failure 401 {object} response.Error
// @Router /api/v1/agents/heartbeat [post]
func (h *Handler) Heartbeat(c *gin.Context) {
	var req HeartbeatInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.AgentID == "" {
		response.BadRequest(c, "agent_id is required")
		return
	}
	if err := h.service.Heartbeat(c.Request.Context(), req); err != nil {
		h.logger.Error("agent heartbeat failed", zap.Error(err))
		response.Internal(c, "failed to store heartbeat")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// List godoc
// @Summary Список воркеров с последним heartbeat
// @Tags agents
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]agents.Agent}
// @Router /api/v1/agents [get]
func (h *Handler) List(c *gin.Context) {
	agents, err := h.service.List(c.Request.Context())
	if err != nil {
		h.logger.Error("list agents failed", zap.Error(err))
		response.Internal(c, "failed to list agents")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": agents})
}
