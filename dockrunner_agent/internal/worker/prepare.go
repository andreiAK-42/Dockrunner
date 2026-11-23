package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"dockrunner_agent/internal/fsx"
	"dockrunner_agent/internal/git"
)

// corruptedProjectDir — папка есть, а .git в ней нет:
// такой проект уже не синхронизировать, только удалять вручную.
func corruptedProjectDir(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return false
	}
	_, err := os.Stat(dir)
	return err == nil
}

func (h *Handler) projectDir(task Task) (string, error) {
	return fsx.ProjectDir(h.workspace, task.Owner, task.AppName)
}

func (h *Handler) prepare(ctx context.Context, task Task) error {
	fail := func(msg string) {
		_ = h.publishEvent(ctx, Event{Type: "prepare.failed", AppID: task.AppID, AppName: task.AppName, DeploymentID: task.DeploymentID, Message: msg})
	}
	dir, err := h.projectDir(task)
	if err != nil {
		fail(err.Error())
		return nil
	}
	h.logger.Info("prepare started", "dir", dir, "repo", task.RepoURL)
	if corruptedProjectDir(dir) {
		fail("папка проекта повреждена (нет .git) — удалите её на воркере: " + dir)
		return nil
	}
	if err := git.EnsureCloned(ctx, task.RepoURL, dir, task.Branch); err != nil {
		fail("репозиторий не склонирован: " + err.Error())
		return err
	}
	dockerfile, compose := git.Detect(dir)
	names := git.ListRoot(dir)
	h.logger.Info("prepare detected", "dir", dir, "dockerfile", dockerfile, "compose", compose, "root", names)
	if dockerfile == "" && compose == "" {
		seen := strings.Join(names, ", ")
		if seen == "" {
			seen = "папка пуста"
		}
		fail("Dockerfile или docker-compose не найден в корне проекта (там: " + seen + ")")
		return nil
	}

	// Detect environment variables from .env and .env.example
	detectedEnvVars, _ := git.DetectEnvFiles(dir)

	// Detect ports from Dockerfile EXPOSE
	var detectedPorts []DetectedPort
	if dockerfile != "" {
		exposedPorts, _ := git.ParseDockerfileExposedPorts(dir, dockerfile)
		for _, p := range exposedPorts {
			detectedPorts = append(detectedPorts, DetectedPort{
				ContainerPort: p.ContainerPort,
				Protocol:      p.Protocol,
				Source:        "dockerfile",
			})
		}
	}

	// Detect ports from docker-compose
	var services []string
	if compose != "" {
		composeServices, err := git.ParseComposeServices(dir, compose)
		if err == nil {
			for _, svc := range composeServices {
				services = append(services, svc.Name)
				for _, p := range svc.Ports {
					detectedPorts = append(detectedPorts, DetectedPort{
						ContainerPort: p.ContainerPort,
						Protocol:      p.Protocol,
						Source:        "compose",
						Service:       svc.Name,
					})
				}
			}
		} else {
			// Fallback to just names if parsing fails
			services, _ = git.ParseComposeNames(dir, compose)
		}
	}

	// Detect branches (local after clone)
	branches, _ := git.ListLocalBranches(dir)

	return h.publishEvent(ctx, Event{
		Type:               "prepare.completed",
		AppID:              task.AppID,
		AppName:            task.AppName,
		DeploymentID:       task.DeploymentID,
		DetectedDockerfile: dockerfile != "",
		ComposeFile:        compose,
		Services:           services,
		Branches:           branches,
		DetectedEnvVars:    detectedEnvVars,
		DetectedPorts:      detectedPorts,
		Message:            "проект готов к сборке",
	})
}

// pullImage тянет готовый образ. Пустая ссылка — ошибка: деплоить нечего.
func (h *Handler) pullImage(ctx context.Context, ref string) (string, error) {
	if ref == "" {
		return "", errNoSource
	}
	if err := h.dock.EnsureImage(ctx, ref); err != nil {
		return "", err
	}
	return ref, nil
}
