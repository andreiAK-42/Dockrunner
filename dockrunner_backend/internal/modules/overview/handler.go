package overview

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"dockrunner_backend/pkg/database/influxdb"
	"dockrunner_backend/pkg/response"
)

type Handler struct {
	influx *influxdb.Client
	logger *zap.Logger
}

func NewHandler(influx *influxdb.Client, logger *zap.Logger) *Handler {
	return &Handler{influx: influx, logger: logger}
}

// Resources godoc
// @Summary Среднесуточная загрузка CPU/RAM кластера
// @Tags overview
// @Produce json
// @Security BearerAuth
// @Param days query int false "Число дней" default(14)
// @Success 200 {object} response.Success{data=[]influxdb.DailyPoint}
// @Router /api/v1/overview/resources [get]
func (h *Handler) Resources(c *gin.Context) {
	days := parseDays(c.Query("days"))
	if h.influx == nil {
		response.OK(c, []influxdb.DailyPoint{})
		return
	}
	points, err := h.influx.QueryClusterDaily(c.Request.Context(), days)
	if err != nil {
		h.logger.Warn("resources query failed", zap.Error(err))
		response.OK(c, []influxdb.DailyPoint{})
		return
	}
	if points == nil {
		points = []influxdb.DailyPoint{}
	}
	response.OK(c, points)
}

func parseDays(raw string) int {
	if raw == "" {
		return 14
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 {
		return 14
	}
	return days
}
