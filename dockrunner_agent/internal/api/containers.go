package api

import (
	"context"
	"net/http"
	"strings"

	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/store"
)

type ContainerItem struct {
	ID        string `json:"id" example:"dep-1048"`
	AppName   string `json:"app_name" example:"frontend-web"`
	Image     string `json:"image" example:"nginx:1.27"`
	Container string `json:"container" example:"a1b2c3d4e5f6"`
	Stored    string `json:"stored_status" example:"running"`
	Live      string `json:"live_status" example:"running"`
}

type ContainersResponse struct {
	Data []ContainerItem `json:"data"`
}

type ContainerSizeItem struct {
	DeploymentID string `json:"deployment_id" example:"dep-1048"`
	ContainerID  string `json:"container_id" example:"a1b2c3d4e5f6"`
	SizeBytes    int64  `json:"size_bytes" example:"104857600"`
}

type ContainerSizesResponse struct {
	Data []ContainerSizeItem `json:"data"`
}

// handleContainers godoc
// @Summary Контейнеры воркера (из SQLite + живой статус)
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Success 200 {object} api.ContainersResponse
// @Failure 401 {object} api.ErrorResponse
// @Router /containers [get]
func (s *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	deployments, err := store.ListDeployments(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	out := make([]ContainerItem, 0, len(deployments))
	for _, d := range deployments {
		live := s.liveStatus(r.Context(), d.Container, d.Status)
		out = append(out, ContainerItem{ID: d.ID, AppName: d.AppName, Image: d.Image, Container: d.Container, Stored: d.Status, Live: live})
	}
	writeJSON(w, http.StatusOK, ContainersResponse{Data: out})
}

// findContainer ищет деплой в SQLite; если записи нет, пытается
// восстановить её из Docker — контейнеры называются <app>-<shortID>,
// поэтому по суффиксу имени находим живой контейнер и пересоздаём запись.
// Это спасает случаи, когда SQLite агента потерян (пересоздание контейнера
// агента без volume), а сами пользовательские контейнеры продолжают работать.
func (s *Server) findContainer(r *http.Request) (*store.Deployment, bool) {
	id := r.PathValue("id")
	dep, err := store.GetDeployment(s.db, id)
	if err == nil {
		// Запись есть, но контейнер мог быть удалён вручную. Тогда пробуем
		// восстановить деплой по живому контейнеру приложения; если Docker
		// недоступен по другой причине — отдаём запись как есть.
		if _, inspectErr := s.dock.InspectSpec(r.Context(), dep.Container); inspectErr == nil || !docker.IsMissing(inspectErr) {
			return dep, true
		}
	}
	// app_name/owner присылает бэкенд: по ним можно найти работающий
	// контейнер, даже если деплой в SQLite записан под другим id.
	recovered := s.recoverDeployment(r.Context(), id, r.URL.Query().Get("app_name"), r.URL.Query().Get("owner"))
	if recovered == nil {
		return nil, false
	}
	return recovered, true
}

// recoverDeployment находит контейнер по docker-метке деплоя (или по
// суффиксу "-<shortID>" для старых контейнеров без меток, или по имени
// приложения) и пересоздаёт минимальную запись деплоя.
// Возвращает nil, если контейнер не найден.
func (s *Server) recoverDeployment(ctx context.Context, id, appName, owner string) *store.Deployment {
	containerID, err := s.dock.FindIDByLabels(ctx, map[string]string{docker.LabelDeployment: id})
	if err != nil || containerID == "" {
		containerID, err = s.dock.FindIDBySuffix(ctx, "-"+store.ShortID(id))
	}
	// Запись в центре может ссылаться на неудачный деплой, в то время как
	// предыдущий контейнер этого же приложения продолжает работать.
	if (err != nil || containerID == "") && appName != "" {
		containerID, err = s.dock.FindIDByApp(ctx, appName, owner)
	}
	if err != nil || containerID == "" {
		s.logger("recovery: container not found", "deployment", id, "app", appName, "error", err)
		return nil
	}
	live, err := s.dock.InspectSpec(ctx, containerID)
	if err != nil {
		s.logger("recovery: inspect failed", "deployment", id, "container", containerID, "error", err)
		return nil
	}
	name := strings.TrimPrefix(live.Name, "/")
	// Имя приложения: явно переданное центром важнее восстановленного по
	// имени контейнера (у пользовательских контейнеров имя произвольное).
	if appName == "" {
		appName = live.Labels[docker.LabelApp]
	}
	if appName == "" {
		appName = name
		if suffix := "-" + store.ShortID(id); strings.HasSuffix(appName, suffix) {
			appName = strings.TrimSuffix(appName, suffix)
		}
	}
	dep := &store.Deployment{
		ID:        id,
		AppName:   appName,
		Name:      name,
		Image:     live.Image,
		Container: containerID,
		Status:    live.Status,
		Owner:     owner,
	}
	// Если запись уже была (просто её контейнер удалён), сохраняем порты,
	// тома, env и репозиторий — иначе refresh пересоздаст контейнер без них.
	if existing, getErr := store.GetDeployment(s.db, id); getErr == nil {
		existing.Container = containerID
		existing.Image = live.Image
		existing.Name = name
		existing.Status = live.Status
		if existing.AppName == "" {
			existing.AppName = appName
		}
		if existing.Owner == "" {
			existing.Owner = owner
		}
		dep = existing
	}
	if err := store.SaveDeployment(s.db, *dep); err != nil {
		s.logger("recovery: save failed", "deployment", id, "error", err)
		return nil
	}
	// Сразу подписываемся на живые логи восстановленного контейнера.
	if dep.Container != "" && dep.Status == "running" {
		s.streamer.Attach(dep.Container, dep.Name)
	}
	s.logger("deployment recovered from docker", "deployment", id, "container", containerID, "app", dep.AppName)
	return dep
}

// liveStatus — живой статус контейнера; при ошибке docker — записанный,
// пропавший контейнер помечается "missing".
func (s *Server) liveStatus(ctx context.Context, containerID, stored string) string {
	live, err := s.dock.ContainerStatus(ctx, containerID)
	if err == nil {
		return live
	}
	if docker.IsMissing(err) {
		return "missing"
	}
	return stored
}

// handleContainerSizes godoc
// @Summary Размеры writable layer всех контейнеров
// @Tags agent
// @Produce json
// @Param Authorization header string false "Bearer <agent_token> (или ?token= в query)"
// @Success 200 {object} api.ContainerSizesResponse
// @Failure 401 {object} api.ErrorResponse
// @Router /containers/sizes [get]
func (s *Server) handleContainerSizes(w http.ResponseWriter, r *http.Request) {
	sizes, err := s.dock.ContainerSizes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to get container sizes: " + err.Error()})
		return
	}
	deployments, err := store.ListDeployments(s.db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list deployments: " + err.Error()})
		return
	}
	// Map container ID -> deployment ID
	containerToDeployment := make(map[string]string)
	for _, d := range deployments {
		if d.Container != "" {
			containerToDeployment[d.Container] = d.ID
		}
	}
	out := make([]ContainerSizeItem, 0, len(sizes))
	for containerID, sizeBytes := range sizes {
		deploymentID := containerToDeployment[containerID]
		if deploymentID == "" {
			deploymentID = "unknown"
		}
		out = append(out, ContainerSizeItem{
			DeploymentID: deploymentID,
			ContainerID:  containerID,
			SizeBytes:    sizeBytes,
		})
	}
	writeJSON(w, http.StatusOK, ContainerSizesResponse{Data: out})
}
