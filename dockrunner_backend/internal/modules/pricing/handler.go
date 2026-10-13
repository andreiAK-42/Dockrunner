package pricing

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
// @Summary Почасовые тарифы всех воркеров
// @Tags pricing
// @Produce json
// @Security BearerAuth
// @Success 200 {object} response.Success{data=[]pricing.Rates}
// @Router /api/v1/pricing [get]
func (h *Handler) List(c *gin.Context) {
	rates, err := h.service.List(c.Request.Context())
	if err != nil {
		h.logger.Error("list pricing failed", zap.Error(err))
		response.Internal(c, "failed to list pricing")
		return
	}
	response.OK(c, rates)
}

type SetRatesRequest struct {
	AgentID        string  `json:"agent_id" binding:"required" example:"worker-01"`
	CpuPerCoreHour float64 `json:"cpu_per_core_hour" example:"1"`
	MemPerGBHour   float64 `json:"mem_per_gb_hour" example:"0.5"`
	DiskPerGBHour  float64 `json:"disk_per_gb_hour" example:"0.1"`
}

// SetRates godoc
// @Summary Задать тарифы воркера (админ)
// @Tags pricing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body pricing.SetRatesRequest true "Тарифы воркера"
// @Success 200 {object} response.Success
// @Router /api/v1/admin/pricing [put]
func (h *Handler) SetRates(c *gin.Context) {
	var req SetRatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	err := h.service.SetRates(c.Request.Context(), Rates{
		AgentID:        req.AgentID,
		CpuPerCoreHour: req.CpuPerCoreHour,
		MemPerGBHour:   req.MemPerGBHour,
		DiskPerGBHour:  req.DiskPerGBHour,
	})
	if err != nil {
		h.logger.Warn("set pricing failed", zap.Error(err))
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"message": "pricing saved"})
}
