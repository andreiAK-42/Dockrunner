package admin

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dockrunner_backend/internal/oplock"
	"dockrunner_backend/pkg/response"
)

type Handler struct {
	service *Service
	logger  *zap.Logger
}

func NewHandler(service *Service, logger *zap.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// GetPlans godoc
// @Summary Тарифные планы (админ)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]admin.Plan}
// @Router /api/v1/admin/plans [get]
func (h *Handler) GetPlans(c *gin.Context) {
	response.OK(c, h.service.Plans())
}

// ListContainers godoc
// @Summary Все контейнеры кластера (админ)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]admin.ContainerRecord}
// @Router /api/v1/admin/containers [get]
func (h *Handler) ListContainers(c *gin.Context) {
	containers, err := h.service.ListContainers(c.Request.Context())
	if err != nil {
		h.logger.Error("list containers failed", zap.Error(err))
		response.Internal(c, "failed to list containers")
		return
	}
	response.OK(c, containers)
}

// ContainerAction godoc
// @Summary Старт/стоп/рестарт чужого контейнера (админ)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID деплоя"
// @Param action path string true "Действие" Enums(stop, restart, remove)
// @Success 200 {object} response.Success
// @Router /api/v1/admin/containers/{id}/{action} [post]
func (h *Handler) ContainerAction(c *gin.Context) {
	action := c.Param("action")
	if action != "stop" && action != "restart" && action != "remove" {
		response.BadRequest(c, "unknown action")
		return
	}
	if err := h.service.ContainerAction(c.Request.Context(), c.Param("id"), action); err != nil {
		h.logger.Error("container action failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		response.Internal(c, "failed to send task")
		return
	}
	response.OK(c, gin.H{"message": "task sent"})
}

// RebootServer godoc
// @Summary Перезагрузить сервер (админ)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID воркера"
// @Success 200 {object} response.Success
// @Router /api/v1/admin/servers/{id}/reboot [post]
func (h *Handler) RebootServer(c *gin.Context) {
	if err := h.service.RebootServer(c.Request.Context(), c.Param("id")); err != nil {
		h.logger.Error("reboot failed", zap.Error(err))
		response.Internal(c, "failed to send task")
		return
	}
	response.OK(c, gin.H{"message": "reboot task sent"})
}

// UpdateContainerParams godoc
// @Summary Обновить параметры запуска контейнера (админ)
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID деплоя"
// @Param body body map[string]any true "Параметры контейнера (image, cpu_cores, memory_gb, disk_gb, ports, env, volumes, restart_policy, expires_at)"
// @Success 200 {object} response.Success
// @Router /api/v1/admin/containers/{id}/params [put]
func (h *Handler) UpdateContainerParams(c *gin.Context) {
	id := c.Param("id")
	var params map[string]any
	if err := c.ShouldBindJSON(&params); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if err := h.service.UpdateContainerParams(c.Request.Context(), id, params); err != nil {
		h.logger.Error("update container params failed", zap.Error(err))
		if errors.Is(err, oplock.ErrBusy) {
			response.Conflict(c, err.Error())
			return
		}
		response.Internal(c, "failed to update container params")
		return
	}
	response.OK(c, gin.H{"message": "container params updated, recreate task sent"})
}
