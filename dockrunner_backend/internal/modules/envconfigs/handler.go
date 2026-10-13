package envconfigs

import (
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

// List godoc
// @Summary Мои env-конфиги
// @Tags env-configs
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]envconfigs.EnvConfig}
// @Router /api/v1/env-configs [get]
func (h *Handler) List(c *gin.Context) {
	configs, err := h.service.List(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("list env configs failed", zap.Error(err))
		response.Internal(c, "failed to list env configs")
		return
	}
	response.OK(c, configs)
}

type EnvConfigRequest struct {
	Name        string   `json:"name" binding:"required" example:"production"`
	Description string   `json:"description" example:"Переменные для прода"`
	Vars        []EnvVar `json:"vars" binding:"required,min=1"`
}

// Create godoc
// @Summary Создать env-конфиг (до 10)
// @Tags env-configs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body envconfigs.EnvConfigRequest true "Название и переменные"
// @Success 201 {object} response.Success{data=envconfigs.EnvConfig}
// @Failure 400 {object} response.Error
// @Router /api/v1/env-configs [post]
func (h *Handler) Create(c *gin.Context) {
	var req EnvConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	config, err := h.service.Create(c.Request.Context(), c.GetString("login"), req.Name, req.Description, req.Vars)
	if err != nil {
		h.logger.Warn("create env config failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.Created(c, config)
}

// Update godoc
// @Summary Обновить env-конфиг
// @Tags env-configs
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID конфига"
// @Param request body envconfigs.EnvConfigRequest true "Название и переменные"
// @Success 200 {object} response.Success{data=envconfigs.EnvConfig}
// @Failure 400 {object} response.Error
// @Router /api/v1/env-configs/{id} [patch]
func (h *Handler) Update(c *gin.Context) {
	var req EnvConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	config, err := h.service.Update(c.Request.Context(), c.GetString("login"), c.Param("id"), req.Name, req.Description, req.Vars)
	if err != nil {
		h.logger.Warn("update env config failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, config)
}

// Delete godoc
// @Summary Удалить env-конфиг
// @Tags env-configs
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID конфига"
// @Success 200 {object} response.Success
// @Router /api/v1/env-configs/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.GetString("login"), c.Param("id")); err != nil {
		h.logger.Warn("delete env config failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}
