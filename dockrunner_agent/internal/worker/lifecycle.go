package worker

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/fsx"
	"dockrunner_agent/internal/queue"
	"dockrunner_agent/internal/store"
)

// loadDeployment достаёт развёртывание из SQLite. allowAppFallback разрешает
// найти контейнер по имени приложения (для start/stop/restart, где нужен
// только container id; для refresh это опасно — потеряются порты/тома).
// Неизвестный id — постоянная ошибка (ретрай не поможет), центр уведомляется.
func (h *Handler) loadDeployment(ctx context.Context, task Task, what string, allowAppFallback bool) (*store.Deployment, error) {
	dep, err := store.GetDeployment(h.db, task.DeploymentID)
	if err == nil {
		return dep, nil
	}
	if errors.Is(err, store.ErrNoDeployment) {
		// Самовосстановление: запись могла потеряться вместе с SQLite,
		// а контейнер в Docker продолжает работать — ищем по docker-метке,
		// затем по имени (для старых контейнеров без меток).
		containerID, findErr := h.dock.FindIDByLabels(ctx, map[string]string{docker.LabelDeployment: task.DeploymentID})
		if findErr != nil || containerID == "" {
			containerID, findErr = h.dock.FindIDBySuffix(ctx, "-"+store.ShortID(task.DeploymentID))
		}
		// Запись в центре может ссылаться на неудачный деплой, а работать
		// может контейнер предыдущего деплоя того же приложения.
		if (findErr != nil || containerID == "") && allowAppFallback && task.AppName != "" {
			containerID, findErr = h.dock.FindIDByApp(ctx, task.AppName, task.Owner)
		}
		if findErr == nil && containerID != "" {
			if live, inspErr := h.dock.InspectSpec(ctx, containerID); inspErr == nil {
				name := strings.TrimPrefix(live.Name, "/")
				appName := live.Labels[docker.LabelApp]
				if appName == "" {
					appName = name
					if suffix := "-" + store.ShortID(task.DeploymentID); strings.HasSuffix(appName, suffix) {
						appName = strings.TrimSuffix(appName, suffix)
					}
				}
				dep := &store.Deployment{
					ID:        task.DeploymentID,
					AppName:   appName,
					Name:      name,
					Image:     live.Image,
					Container: containerID,
					Status:    live.Status,
				}
				if saveErr := store.SaveDeployment(h.db, *dep); saveErr == nil {
					h.logger.Info("deployment recovered from docker", "deployment", task.DeploymentID, "container", containerID)
					return dep, nil
				}
			}
		}
		return nil, h.missingDeployment(ctx, task, what)
	}
	return nil, err
}

func (h *Handler) refresh(ctx context.Context, task Task) error {
	dep, err := h.loadDeployment(ctx, task, "обновление", false)
	if err != nil {
		return err
	}
	owner := dep.Owner
	if owner == "" {
		owner = task.Owner
	}
	var ports []PortTask
	_ = json.Unmarshal([]byte(dep.Ports), &ports)
	var volumes []VolumeTask
	_ = json.Unmarshal([]byte(dep.Volumes), &volumes)
	var env map[string]string
	_ = json.Unmarshal([]byte(dep.Env), &env)
	// Сервис-контейнер compose пересобирается через родительскую задачу,
	// чтобы clone/detect шли в правильную папку проекта. Обновление одного
	// сервиса пересобирает все сервисы проекта: проект тянется из git целиком.
	if dep.ParentID != "" && dep.Service != "" {
		parentApp := parentAppName(dep)
		services, projectPorts := h.projectServices(dep)
		if len(ports) == 0 {
			ports = projectPorts
		}
		return h.deploy(ctx, Task{
			DeploymentID: dep.ParentID,
			AppID:        dep.AppID,
			AppName:      parentApp,
			Owner:        owner,
			RepoURL:      dep.RepoURL,
			Services:     services,
			Ports:        ports,
			Volumes:      volumes,
			Env:          env,
			Restart:      "unless-stopped",
			CPUCores:     dep.CPUCores,
			MemoryGB:     dep.MemoryGB,
			DiskGB:       dep.DiskGB,
			Branch:       dep.Branch,
		})
	}
	return h.deploy(ctx, Task{
		DeploymentID:  dep.ID,
		AppID:         dep.AppID,
		AppName:       dep.AppName,
		Owner:         owner,
		RepoURL:       dep.RepoURL,
		Image:         dep.Image,
		ContainerName: dep.Name,
		Ports:         ports,
		Volumes:       volumes,
		Env:           env,
		Restart:       "unless-stopped",
		CPUCores:      dep.CPUCores,
		MemoryGB:      dep.MemoryGB,
		DiskGB:        dep.DiskGB,
		Branch:        dep.Branch,
	})
}

func (h *Handler) stop(ctx context.Context, task Task) error {
	dep, err := h.loadDeployment(ctx, task, "остановка", true)
	if err != nil {
		return err
	}
	if err := h.dock.Stop(ctx, dep.Container); err != nil {
		if docker.IsMissing(err) {
			// Контейнера уже нет — цель достигнута, сообщаем как остановку.
			_ = store.UpdateDeploymentStatus(h.db, task.DeploymentID, "stopped", dep.Container)
			return h.publishStopped(ctx, task, dep)
		}
		h.logger.Warn("stop failed", "deployment", task.DeploymentID, "error", err)
		_ = h.publishEvent(ctx, appEvent("op.failed", task, "остановка не удалась: "+err.Error()))
		return queue.Permanent(err)
	}
	_ = store.UpdateDeploymentStatus(h.db, task.DeploymentID, "stopped", dep.Container)
	return h.publishStopped(ctx, task, dep)
}

// publishStopped шлёт container.stopped и вызывает detach логов.
// Ошибка публикации означает, что централь не узнает об остановке.
func (h *Handler) publishStopped(ctx context.Context, task Task, dep *store.Deployment) error {
	err := h.publishEvent(ctx, appEvent("container.stopped", task, "приложение остановлено"))
	if h.onStopped != nil {
		h.onStopped(dep.Container)
	}
	return err
}

func (h *Handler) restart(ctx context.Context, task Task) error {
	dep, err := h.loadDeployment(ctx, task, "перезапуск", true)
	if err != nil {
		return err
	}
	if err := h.dock.Restart(ctx, dep.Container); err != nil {
		if docker.IsMissing(err) {
			// Контейнер исчез (удалён вручную и т.п.) — restart не имеет смысла.
			_ = store.UpdateDeploymentStatus(h.db, task.DeploymentID, "stopped", dep.Container)
			return h.publishEvent(ctx, appEvent("container.stopped", task, "контейнер не найден на узле — приложение остановлено"))
		}
		h.logger.Warn("restart failed", "deployment", task.DeploymentID, "error", err)
		_ = h.publishEvent(ctx, appEvent("op.failed", task, "перезапуск не удался: "+err.Error()))
		return queue.Permanent(err)
	}
	_ = store.UpdateDeploymentStatus(h.db, task.DeploymentID, "running", dep.Container)
	return h.publishEvent(ctx, appEvent("container.restarted", task, "приложение перезапущено"))
}

func (h *Handler) remove(ctx context.Context, task Task) error {
	dep, err := store.GetDeployment(h.db, task.DeploymentID)
	if err != nil {
		if errors.Is(err, store.ErrNoDeployment) {
			// Idempotent delete: nothing stored locally means there is
			// nothing to stop — report success so central drops the record.
			h.logger.Info("remove of unknown deployment, treating as done", "deployment", task.DeploymentID)
			return h.publishEvent(ctx, appEvent("container.removed", task, "приложение удалено"))
		}
		return err
	}
	removeErr := h.dock.Remove(ctx, dep.Container)
	if task.RemoveVolumes {
		removeErr = h.dock.RemoveWithVolumes(ctx, dep.Container)
	}
	if removeErr != nil && !docker.IsMissing(removeErr) {
		h.logger.Warn("remove failed", "deployment", task.DeploymentID, "error", removeErr)
		_ = h.publishEvent(ctx, appEvent("op.failed", task, "удаление не удалось: "+removeErr.Error()))
		return queue.Permanent(removeErr)
	}
	// Общие ресурсы (тома, сеть, папка) чистим под одним замком: иначе
	// параллельные remove успевают увидеть друг друга и ничего не удаляют.
	h.removeMu.Lock()
	_ = store.DeleteDeployment(h.db, task.DeploymentID)
	removedVolumes := h.cleanupProjectResources(ctx, dep, task.RemoveVolumes)
	h.removeProjectDir(dep)
	h.removeMu.Unlock()

	if h.onStopped != nil {
		h.onStopped(dep.Container)
	}
	// Терминальное событие обязано уйти — иначе централь хранит запись
	// и держит операционный замок до TTL.
	ev := appEvent("container.removed", task, "приложение удалено")
	ev.RemovedVolumes = removedVolumes
	return h.publishEvent(ctx, ev)
}

// cleanupProjectResources удаляет общие ресурсы, у которых больше нет
// владельца: тома истёкшего деплоя и сеть compose-проекта. Возвращает имена
// реально удалённых томов, чтобы центр убрал их из своего списка.
func (h *Handler) cleanupProjectResources(ctx context.Context, dep *store.Deployment, removeVolumes bool) []string {
	var removed []string
	if removeVolumes {
		removed = h.removeUnusedVolumes(ctx, dep)
	}
	h.removeComposeNetwork(ctx, dep)
	return removed
}

// removeUnusedVolumes удаляет именованные тома деплоя, если их больше не
// монтирует ни одна оставшаяся запись. Пути хоста не трогаем, а анонимные
// тома Docker удаляет вместе с контейнером.
func (h *Handler) removeUnusedVolumes(ctx context.Context, dep *store.Deployment) []string {
	var volumes []VolumeTask
	if err := json.Unmarshal([]byte(dep.Volumes), &volumes); err != nil {
		return nil
	}
	remaining, err := store.ListDeployments(h.db)
	if err != nil {
		h.logger.Warn("failed to list deployments for volume cleanup", "error", err)
		return nil
	}
	removed := make([]string, 0, len(volumes))
	for _, volume := range volumes {
		if volume.Name == "" || volume.HostPath != "" || volumeInUse(remaining, volume.Name) {
			continue
		}
		name, err := sanitizeVolumeName(volume.Name)
		if err != nil {
			continue
		}
		if err := h.dock.VolumeRemove(ctx, name); err != nil && !docker.IsMissing(err) {
			h.logger.Warn("failed to remove volume", "volume", name, "error", err)
			continue
		}
		removed = append(removed, name)
	}
	return removed
}

// volumeInUse сообщает, монтирует ли имя тома какая-нибудь оставшаяся запись.
func volumeInUse(deployments []store.Deployment, name string) bool {
	for _, dep := range deployments {
		var volumes []VolumeTask
		if err := json.Unmarshal([]byte(dep.Volumes), &volumes); err != nil {
			continue
		}
		for _, volume := range volumes {
			if volume.Name == name && volume.HostPath == "" {
				return true
			}
		}
	}
	return false
}

// composeNetworkName возвращает общую сеть compose-проекта.
func composeNetworkName(dep *store.Deployment) (string, bool) {
	if dep.ParentID == "" || dep.Service == "" {
		return "", false
	}
	app := parentAppName(dep)
	name, err := sanitizeContainerName(app, app+"-net", dep.ParentID)
	if err != nil {
		return "", false
	}
	return name, true
}

// removeComposeNetwork удаляет сеть проекта, когда в ней не осталось его
// сервисов. Docker не даст удалить сеть с подключёнными концами, поэтому
// оставшийся сервис просто сохранит её до своего удаления.
func (h *Handler) removeComposeNetwork(ctx context.Context, dep *store.Deployment) {
	netName, ok := composeNetworkName(dep)
	if !ok {
		return
	}
	remaining, err := store.ListDeployments(h.db)
	if err != nil {
		h.logger.Warn("failed to list deployments for network cleanup", "error", err)
		return
	}
	app := parentAppName(dep)
	for _, other := range remaining {
		if other.ParentID == "" || other.Service == "" {
			continue
		}
		if parentAppName(&other) == app {
			return
		}
	}
	if err := h.dock.NetworkRemove(ctx, netName); err != nil && !docker.IsMissing(err) {
		h.logger.Warn("failed to remove compose network", "network", netName, "error", err)
		return
	}
	h.logger.Info("compose network removed", "network", netName)
}

// parentAppName returns the app that owns the project folder. For compose
// services the stored ParentApp is authoritative; old records fall back to
// trimming the service suffix from «app-service».
func parentAppName(dep *store.Deployment) string {
	if dep.ParentApp != "" {
		return dep.ParentApp
	}
	if dep.ParentID != "" && dep.Service != "" {
		return strings.TrimSuffix(dep.AppName, "-"+dep.Service)
	}
	return dep.AppName
}

// projectServices lists every compose service of the same project that is
// currently deployed, so a refresh rebuilds the whole project instead of one
// service. It also returns the project ports: they belong to the first service
// in compose order, which may not be the service the user opened.
func (h *Handler) projectServices(dep *store.Deployment) ([]string, []PortTask) {
	siblings, err := store.ListDeploymentsByParent(h.db, dep.ParentID)
	if err != nil {
		h.logger.Warn("failed to list project services", "parent", dep.ParentID, "error", err)
		return []string{dep.Service}, nil
	}
	services := make([]string, 0, len(siblings))
	seen := make(map[string]bool, len(siblings))
	for _, sibling := range siblings {
		if sibling.Service == "" || seen[sibling.Service] {
			continue
		}
		seen[sibling.Service] = true
		services = append(services, sibling.Service)
	}
	if len(services) == 0 {
		return []string{dep.Service}, nil
	}
	var ports []PortTask
	for _, sibling := range siblings {
		var candidate []PortTask
		if err := json.Unmarshal([]byte(sibling.Ports), &candidate); err == nil && len(candidate) > 0 {
			ports = candidate
			break
		}
	}
	return services, ports
}

// removeProjectDir deletes the workspace folder of the deployment unless
// another deployment still uses it. Compose services share the parent app
// folder, and a redeploy of the same app reuses it.
func (h *Handler) removeProjectDir(dep *store.Deployment) {
	app := parentAppName(dep)
	if app == "" {
		return
	}
	remaining, err := store.CountDeploymentsUsingProject(h.db, dep.Owner, app)
	if err != nil {
		h.logger.Warn("failed to check project dir usage", "app", app, "error", err)
		return
	}
	if remaining > 0 {
		h.logger.Info("project directory kept, still used by other deployments", "app", app, "remaining", remaining)
		return
	}
	projDir, err := fsx.ProjectDir(h.workspace, dep.Owner, app)
	if err != nil {
		return
	}
	if err := os.RemoveAll(projDir); err != nil {
		h.logger.Warn("failed to remove project dir", "dir", projDir, "error", err)
		return
	}
	h.logger.Info("project directory removed", "dir", projDir)
}

func (h *Handler) volumeRemove(ctx context.Context, task Task) error {
	name, err := sanitizeVolumeName(task.VolumeName)
	if err != nil {
		return queue.Permanent(err)
	}
	return h.dock.VolumeRemove(ctx, name)
}

func (h *Handler) reboot(ctx context.Context) error {
	if !canReboot() {
		_ = h.publishEvent(ctx, Event{Type: "server.reboot_failed", Message: "перезагрузка не поддерживается"})
		return errRebootUnsupported
	}
	_ = h.publishEvent(ctx, Event{Type: "server.rebooting", Message: "сервер перезагружается"})
	go delayedReboot(h.logger)
	return nil
}
