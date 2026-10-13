package servers

import (
	"strconv"

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
// @Summary Список серверов
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param page query int false "Страница" default(1)
// @Param limit query int false "Размер страницы" default(20)
// @Success 200 {object} response.Success{data=[]servers.Server}
// @Router /api/v1/servers [get]
func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	servers, total, err := h.service.List(c.Request.Context(), page, limit)
	if err != nil {
		h.logger.Error("list servers failed", zap.Error(err))
		response.Internal(c, "failed to list servers")
		return
	}
	response.Paginated(c, servers, total, page, limit)
}

// GetByID godoc
// @Summary Сервер по id
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID сервера"
// @Success 200 {object} response.Success{data=servers.Server}
// @Failure 404 {object} response.Error
// @Router /api/v1/servers/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")
	srv, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if srv == nil {
		response.NotFound(c, "server not found")
		return
	}
	response.OK(c, srv)
}

type CreateRequest struct {
	Name   string `json:"name" binding:"required,min=2,max=64" example:"worker-07"`
	IP     string `json:"ip" binding:"required,ip" example:"10.0.0.7"`
	Region string `json:"region" binding:"required" example:"Frankfurt"`
}

// Create godoc
// @Summary Добавить сервер
// @Tags servers
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body servers.CreateRequest true "Данные сервера"
// @Success 201 {object} response.Success{data=servers.Server}
// @Failure 400 {object} response.Error
// @Router /api/v1/servers [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	srv, err := h.service.Create(c.Request.Context(), req.Name, req.IP, req.Region)
	if err != nil {
		h.logger.Error("create server failed", zap.Error(err))
		response.Internal(c, "failed to create server")
		return
	}
	response.Created(c, srv)
}

// Delete godoc
// @Summary Удалить сервер
// @Tags servers
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID сервера"
// @Success 204 "Удалён"
// @Router /api/v1/servers/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("delete server failed", zap.Error(err))
		response.Internal(c, "failed to delete server")
		return
	}
	c.Status(204)
}
