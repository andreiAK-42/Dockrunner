package worker

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/git"
	"dockrunner_agent/internal/store"
)

// buildComposeService собирает образ одного compose-сервиса:
// свой Dockerfile/context — свой тег, готовый image — pull.
func (h *Handler) buildComposeService(ctx context.Context, task Task, svc git.ComposeService) (string, error) {
	if svc.Dockerfile != "" {
		tag := "app-" + shortID(task.DeploymentID) + "-" + svc.Name + ":latest"
		// Панель показывает запись «приложение-сервис», поэтому лог сборки
		// пишем под её id, а не под id родительского деплоя.
		buildTask := task
		buildTask.DeploymentID = task.DeploymentID + "-" + svc.Name
		if err := h.dock.Build(ctx, svc.ContextDir, svc.Dockerfile, tag, task.Env, h.buildLines(buildTask)); err != nil {
			return "", err
		}
		return tag, nil
	}
	if svc.Image != "" {
		if err := h.dock.EnsureImage(ctx, svc.Image); err != nil {
			return "", err
		}
		return svc.Image, nil
	}
	return "", errComposeEmpty
}

func (h *Handler) buildDockerfile(ctx context.Context, task Task, contextDir, dockerfile, tag string) error {
	h.logger.Info("deploy build started", "deployment", task.DeploymentID, "dockerfile", dockerfile)
	start := time.Now()
	if err := h.dock.Build(ctx, contextDir, dockerfile, tag, task.Env, h.buildLines(task)); err != nil {
		h.logger.Warn("deploy build failed", "deployment", task.DeploymentID, "error", err)
		return err
	}
	h.logger.Info("deploy build finished", "deployment", task.DeploymentID, "took", time.Since(start).Round(time.Second).String())
	return nil
}

// failChild уведомляет центр о провале одного сервис-контейнера.
func (h *Handler) failChild(ctx context.Context, task Task, parentApp, deploymentID, appName, message string, err error) error {
	ev := appEvent("deploy.failed", task, message)
	ev.DeploymentID = deploymentID
	ev.AppName = appName
	ev.ParentApp = parentApp
	_ = h.publishEvent(ctx, ev)
	return err
}

// buildLines складывает строки сборки в buildlog и подсвечивает ERROR в логах.
func (h *Handler) buildLines(task Task) func(line string, progress bool) {
	return func(line string, progress bool) {
		h.builds.Append(task.DeploymentID, line)
		if progress {
			return
		}
		if strings.HasPrefix(line, "ERROR:") {
			h.logger.Warn("deploy build", "deployment", task.DeploymentID, "line", line)
			return
		}
		h.logger.Info("deploy build", "deployment", task.DeploymentID, "line", line)
	}
}

// failDeploy уведомляет центр о провале и возвращает ошибку дальше.
func (h *Handler) failDeploy(ctx context.Context, task Task, message string, err error) error {
	_ = h.publishEvent(ctx, appEvent("deploy.failed", task, message))
	return err
}

func (h *Handler) deploy(ctx context.Context, task Task) error {
	h.logger.Info("deploy started", "deployment", task.DeploymentID, "app", task.AppName, "repo", task.RepoURL)
	// Прямой запуск готового образа (refresh, повторный прогон): один контейнер.
	if task.RepoURL == "" {
		image, err := h.pullImage(ctx, task.Image)
		if err != nil {
			return h.failDeploy(ctx, task, "образ не скачан: "+err.Error(), err)
		}
		return h.deploySingle(ctx, task, image, task.AppName, task.DeploymentID, task.Ports, task.ContainerName, "", "", "", "")
	}
	dir, err := h.projectDir(task)
	if err != nil {
		return h.failDeploy(ctx, task, err.Error(), err)
	}
	if err := git.EnsureCloned(ctx, task.RepoURL, dir, task.Branch); err != nil {
		return h.failDeploy(ctx, task, "репозиторий не склонирован: "+err.Error(), err)
	}
	dockerfile, compose := git.Detect(dir)
	if dockerfile == "" && compose == "" {
		return h.failDeploy(ctx, task, errNoDockerfile.Error(), errNoDockerfile)
	}
	// Режим явно выбран пользователем. Для старых задач без build_mode сохраняем прежнее автоопределение.
	mode := task.BuildMode
	if mode == "" {
		if compose != "" && len(task.Services) > 0 {
			mode = "compose"
		} else {
			mode = "dockerfile"
		}
	}
	if mode == "dockerfile" {
		if dockerfile == "" {
			return h.failDeploy(ctx, task, "выбран Dockerfile, но файл не найден", errNoDockerfile)
		}
		tag := "app-" + shortID(task.DeploymentID) + ":latest"
		// Запись «building» появляется до начала сборки: иначе API агента
		// отвечает центру 404 всё время сборки, и централь помечает
		// контейнер «stopped», хотя образ ещё собирается.
		name, _ := sanitizeContainerName(task.AppName, task.ContainerName, task.DeploymentID)
		h.markBuilding(task, task.Ports, task.DeploymentID, task.AppName, name, tag, "", "", "")
		if err := h.buildDockerfile(ctx, task, dir, dockerfile, tag); err != nil {
			return h.failDeploy(ctx, task, "образ не собран: "+err.Error(), err)
		}
		return h.deploySingle(ctx, task, tag, task.AppName, task.DeploymentID, task.Ports, task.ContainerName, "", "", "", "")
	}
	if mode == "compose" {
		if compose == "" {
			return h.failDeploy(ctx, task, "выбран Docker Compose, но compose-файл не найден", errNoDockerfile)
		}
		if len(task.Services) == 0 {
			return h.failDeploy(ctx, task, "для Docker Compose необходимо выбрать хотя бы один сервис", errComposeEmpty)
		}
		return h.deployCompose(ctx, task, dir, compose)
	}
	return h.failDeploy(ctx, task, "неизвестный режим сборки: "+mode, errNoDockerfile)
}

// markBuilding сохраняет запись о деплое со статусом "building" до начала
// сборки/запуска, чтобы API агента уже знал о нём и отвечал сохранённой
// спецификацией вместо 404. Без этого централь помечал контейнер «stopped»
// в первые же секунды сборки. Существующая запись перечитывается, чтобы не
// потерять поля, которых нет в задаче. Ссылка на старый контейнер
// сохраняется до успешного запуска новой версии.
func (h *Handler) markBuilding(task Task, ports []PortTask, deploymentID, appName, name, image, parentID, parentApp, service string) {
	portsJSON, _ := json.Marshal(ports)
	volumesJSON, _ := json.Marshal(task.Volumes)
	envJSON, _ := json.Marshal(task.Env)
	dep, err := store.GetDeployment(h.db, deploymentID)
	if err != nil {
		dep = &store.Deployment{ID: deploymentID}
	}
	dep.AppID = task.AppID
	dep.AppName = appName
	dep.Owner = task.Owner
	dep.RepoURL = task.RepoURL
	dep.Name = name
	dep.Image = image
	// Container намеренно не сбрасываем: до успешного запуска новой версии
	// продолжает работать старый контейнер, и запись должна на него
	// ссылаться. Иначе во время сборки (и после её провала) приложение
	// «пропадает» из панели, хотя контейнер жив.
	dep.Status = "building"
	dep.Ports = string(portsJSON)
	dep.Volumes = string(volumesJSON)
	dep.Env = string(envJSON)
	dep.CPUCores = task.CPUCores
	dep.MemoryGB = task.MemoryGB
	dep.ParentID = parentID
	dep.ParentApp = parentApp
	dep.Service = service
	dep.Branch = task.Branch
	if err := store.SaveDeployment(h.db, *dep); err != nil {
		h.logger.Warn("building record not saved", "deployment", deploymentID, "error", err)
	}
}

// deploySingle запускает один контейнер с лимитами и сообщает центру.
// parentID/parentApp/service заполнены только для сервис-контейнеров compose.
func (h *Handler) deploySingle(ctx context.Context, task Task, image, appName, deploymentID string, ports []PortTask, customName, network, parentID, parentApp, service string) error {
	h.logger.Info("deploy image ready", "deployment", deploymentID, "image", image)
	volumes, err := h.resolveVolumes(task)
	if err != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "тома неверны: "+err.Error(), err)
	}
	mapped, err := toPortMappings(ports)
	if err != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "неверный порт", err)
	}
	name, err := sanitizeContainerName(appName, customName, deploymentID)
	if err != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, err.Error(), err)
	}
	h.markBuilding(task, ports, deploymentID, appName, name, image, parentID, parentApp, service)
	portsJSON, _ := json.Marshal(ports)
	volumesJSON, _ := json.Marshal(task.Volumes)
	envJSON, _ := json.Marshal(task.Env)
	// Старый контейнер не удаляем сразу: останавливаем и переименовываем в
	// резерв, чтобы неудачный запуск новой версии не оставил приложение без
	// работающего контейнера (порты/тома уже заняты его конфигурацией).
	backupName := name + "-dockrunner-backup"
	oldID, findErr := h.dock.FindIDByName(ctx, name)
	if findErr != nil {
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "старый контейнер не найден: "+findErr.Error(), findErr)
	}
	if oldID != "" {
		// Остаток прошлой неудачной попытки замены.
		_ = h.dock.Remove(ctx, backupName)
		if err := h.dock.Stop(ctx, oldID); err != nil && !docker.IsMissing(err) {
			return h.failChild(ctx, task, parentApp, deploymentID, appName, "старый контейнер не остановлен: "+err.Error(), err)
		}
		if err := h.dock.Rename(ctx, oldID, backupName); err != nil {
			// Не смогли освободить имя — новый контейнер не создать.
			_ = h.dock.Start(ctx, oldID)
			return h.failChild(ctx, task, parentApp, deploymentID, appName, "старый контейнер не переименован: "+err.Error(), err)
		}
	}
	containerID, err := h.dock.Run(ctx, docker.RunSpec{
		Name:          name,
		Image:         image,
		Env:           task.Env,
		Ports:         mapped,
		Volumes:       volumes,
		RestartPolicy: task.Restart,
		CPUCores:      task.CPUCores,
		MemoryGB:      task.MemoryGB,
		DiskGB:        task.DiskGB, // лимит диска (overlay2.size)
		Network:       network,
		DeploymentID:  deploymentID,
		AppName:       appName,
		Owner:         task.Owner,
	})
	if err != nil {
		// Run мог создать контейнер и упасть на старте — освобождаем имя
		// и возвращаем старую версию, чтобы приложение продолжало работать.
		if oldID != "" {
			_ = h.dock.Remove(ctx, name)
			if renameErr := h.dock.Rename(ctx, backupName, name); renameErr == nil {
				_ = h.dock.Start(ctx, oldID)
			}
		}
		h.builds.Append(deploymentID, "ERROR: запуск не удался: "+err.Error())
		h.logger.Warn("deploy run failed", "deployment", deploymentID, "error", err)
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "запуск не удался: "+err.Error(), err)
	}
	// Новая версия запустилась — резерв больше не нужен.
	if oldID != "" {
		_ = h.dock.Remove(ctx, backupName)
	}
	h.logger.Info("deploy completed", "deployment", deploymentID, "container", containerID)

	if err := store.SaveDeployment(h.db, store.Deployment{
		ID:        deploymentID,
		AppID:     task.AppID,
		AppName:   appName,
		Owner:     task.Owner,
		RepoURL:   task.RepoURL,
		Name:      name,
		Image:     image,
		Container: containerID,
		Status:    "running",
		Ports:     string(portsJSON),
		Volumes:   string(volumesJSON),
		Env:       string(envJSON),
		CPUCores:  task.CPUCores,
		MemoryGB:  task.MemoryGB,
		ParentID:  parentID,
		ParentApp: parentApp,
		Service:   service,
		Branch:    task.Branch,
	}); err != nil {
		// Без записи в SQLite центр не сможет получить spec/логи/статус —
		// лучше громко упасть, чем молча потерять деплой.
		h.logger.Error("deployment record not saved", "deployment", deploymentID, "error", err)
		return h.failChild(ctx, task, parentApp, deploymentID, appName, "запись деплоя не сохранена: "+err.Error(), err)
	}

	ev := appEvent("deploy.completed", task, "приложение запущено")
	ev.DeploymentID = deploymentID
	ev.AppName = appName
	ev.Image = image
	ev.Ports = ports
	ev.ParentApp = parentApp
	if err := h.publishEvent(ctx, ev); err != nil {
		return err
	}

	if h.onReady != nil {
		h.onReady(containerID, name)
	}
	return nil
}

// deployCompose запускает каждый сервис compose отдельным контейнером
// «приложение-сервис» в общей сети. Лимиты из заявки действуют на КАЖДЫЙ
// контейнер, пользовательские порты отдаются первому сервису.
func (h *Handler) deployCompose(ctx context.Context, task Task, dir, compose string) error {
	all, err := git.ParseComposeServices(dir, compose)
	if err != nil {
		return h.failDeploy(ctx, task, "compose не разобран: "+err.Error(), err)
	}
	svcs := all
	if len(task.Services) > 0 {
		requested := map[string]bool{}
		for _, name := range task.Services {
			requested[name] = true
		}
		filtered := make([]git.ComposeService, 0, len(task.Services))
		for _, svc := range all {
			if requested[svc.Name] {
				filtered = append(filtered, svc)
			}
		}
		svcs = filtered
		if len(svcs) == 0 {
			return h.failDeploy(ctx, task, "запрошенные сервисы не найдены в compose", errComposeEmpty)
		}
	}
	netName, _ := sanitizeContainerName(task.AppName, task.AppName+"-net", task.DeploymentID)
	if err := h.dock.EnsureNetwork(ctx, netName); err != nil {
		return h.failDeploy(ctx, task, "сеть не создана: "+err.Error(), err)
	}
	failed := 0
	for i, svc := range svcs {
		childApp := task.AppName + "-" + svc.Name
		childID := task.DeploymentID + "-" + svc.Name
		var ports []PortTask
		if i == 0 {
			ports = task.Ports
		}
		childName, _ := sanitizeContainerName(childApp, childApp, childID)
		h.markBuilding(task, ports, childID, childApp, childName, "", task.DeploymentID, task.AppName, svc.Name)
		image, err := h.buildComposeService(ctx, task, svc)
		if err != nil {
			failed++
			_ = h.failChild(ctx, task, task.AppName, childID, childApp, "сервис "+svc.Name+": "+err.Error(), err)
			continue
		}
		if err := h.deploySingle(ctx, task, image, childApp, childID, ports, childApp, netName, task.DeploymentID, task.AppName, svc.Name); err != nil {
			failed++
		}
	}
	if failed == len(svcs) {
		return h.failDeploy(ctx, task, "ни один сервис не запустился", errComposeEmpty)
	}
	return nil
}
