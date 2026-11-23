package api

import "net/http"

type StatsResponse struct {
	CPUPercent float64 `json:"cpu_percent" example:"12.5"`
	MemMB      float64 `json:"mem_mb" example:"256.4"`
}

// handleContainerStats godoc
// @Summary Живая статистика контейнера (CPU %, RAM МБ)
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Param id path string true "ID деплоя"
// @Success 200 {object} api.StatsResponse
// @Failure 401 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /containers/{id}/stats [get]
func (s *Server) handleContainerStats(w http.ResponseWriter, r *http.Request) {
	dep, ok := s.findContainer(r)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "deployment not found"})
		return
	}
	// Сборка идёт, контейнера ещё нет — нулевая статистика вместо ошибки.
	if dep.Container == "" {
		writeJSON(w, http.StatusOK, StatsResponse{CPUPercent: 0, MemMB: 0})
		return
	}
	stats, err := s.dock.Stats(r.Context(), dep.Container)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "stats unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, StatsResponse{CPUPercent: stats.CPUPercent, MemMB: stats.MemMB})
}
