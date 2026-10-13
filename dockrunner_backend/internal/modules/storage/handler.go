package storage

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

// Usage godoc
// @Summary Занятое место пользователя: контейнеры + личные папки
// @Tags storage
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=storage.Usage}
// @Router /api/v1/storage/usage [get]
func (h *Handler) Usage(c *gin.Context) {
	usage, err := h.service.Usage(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("storage usage failed", zap.Error(err))
		response.Internal(c, "failed to load usage")
		return
	}
	response.OK(c, usage)
}
