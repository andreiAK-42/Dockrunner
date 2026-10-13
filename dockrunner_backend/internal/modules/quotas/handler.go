package quotas

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

// Get godoc
// @Summary Мои лимиты: использовано и лимиты тарифа
// @Tags quotas
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=quotas.Quotas}
// @Router /api/v1/quotas [get]
func (h *Handler) Get(c *gin.Context) {
	quotas, err := h.service.Get(c.Request.Context(), c.GetString("login"))
	if err != nil {
		h.logger.Error("quotas failed", zap.Error(err))
		response.Internal(c, "failed to load quotas")
		return
	}
	response.OK(c, quotas)
}
