package volumes

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
// @Summary Мои тома
// @Tags volumes
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]volumes.Volume}
// @Router /api/v1/volumes [get]
func (h *Handler) List(c *gin.Context) {
	volumes, err := h.service.List(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("list volumes failed", zap.Error(err))
		response.Internal(c, "failed to list volumes")
		return
	}
	response.OK(c, volumes)
}

type CreateRequest struct {
	Name          string  `json:"name" binding:"required" example:"pg-data"`
	NodeID        string  `json:"node_id" binding:"required" example:"worker-01"`
	SizeGB        float64 `json:"size_gb" example:"20"`
	ContainerPath string  `json:"container_path" example:"/var/lib/postgresql/data"`
}

// Create godoc
// @Summary Создать том
// @Tags volumes
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body volumes.CreateRequest true "Название, узел, размер"
// @Success 201 {object} response.Success{data=volumes.Volume}
// @Failure 400 {object} response.Error
// @Router /api/v1/volumes [post]
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	volume, err := h.service.Create(c.Request.Context(), c.GetString("login"), req.NodeID, req.Name, req.SizeGB, req.ContainerPath)
	if err != nil {
		h.logger.Error("create volume failed", zap.Error(err))
		response.Internal(c, "failed to create volume")
		return
	}
	response.Created(c, volume)
}

// Delete godoc
// @Summary Удалить том с воркера
// @Tags volumes
// @Produce json
// @Security BearerAuth
// @Param name path string true "Имя тома"
// @Param node query string true "ID воркера"
// @Success 200 {object} response.Success
// @Router /api/v1/volumes/{name} [delete]
func (h *Handler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.GetString("login"), c.Query("node"), c.Param("name")); err != nil {
		h.logger.Warn("delete volume failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "deleted"})
}
