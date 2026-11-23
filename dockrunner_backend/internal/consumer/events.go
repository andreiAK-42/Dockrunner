package consumer

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
	"dockrunner_backend/internal/modules/containers"
	"dockrunner_backend/internal/realtime"
	"dockrunner_backend/pkg/queue"
)

func StartEvents(ctx context.Context, q *queue.Client, adminService *admin.Service, containersService *containers.Service, hub *realtime.Hub, logger *zap.Logger) error {
	return q.Consume(queue.QueueEvents, func(ctx context.Context, body []byte) error {
		var event admin.AgentEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		switch event.Type {
		case "prepare.completed":
			// Convert admin.DetectedPort to containers.DetectedPort
			detectedPorts := make([]containers.DetectedPort, len(event.DetectedPorts))
			for i, p := range event.DetectedPorts {
				detectedPorts[i] = containers.DetectedPort{
					ContainerPort: p.ContainerPort,
					Protocol:      p.Protocol,
					Source:        p.Source,
					Service:       p.Service,
				}
			}
			containersService.ResolveCheck(event.DeploymentID, true, event.DetectedDockerfile, event.ComposeFile, event.Services, event.Branches, event.DetectedEnvVars, detectedPorts, "")
			logger.Info("prepare resolved", zap.String("deployment", event.DeploymentID), zap.Bool("dockerfile", event.DetectedDockerfile), zap.String("compose", event.ComposeFile), zap.Strings("services", event.Services), zap.Strings("branches", event.Branches), zap.Any("env_vars", event.DetectedEnvVars), zap.Any("ports", detectedPorts))
			return nil
		case "prepare.failed":
			containersService.ResolveCheck(event.DeploymentID, false, false, "", nil, nil, nil, nil, event.Message)
			logger.Info("prepare resolved", zap.String("deployment", event.DeploymentID), zap.String("failed", event.Message))
			return nil
		}

		if err := adminService.ApplyEvent(ctx, event); err != nil {
			logger.Warn("apply agent event failed", zap.Error(err))
			return err
		}

		publishContainerEvent(hub, event)
		releaseOpForTerminal(event, containersService)
		return nil
	})
}

// publishContainerEvent pushes the terminal state of a container to the
// owner's browser and to administrators. Check events are published by the
// containers service itself (ResolveCheck).
func publishContainerEvent(hub *realtime.Hub, event admin.AgentEvent) {
	payload := map[string]any{
		"id":          event.DeploymentID,
		"app_id":      event.AppID,
		"app_name":    event.AppName,
		"owner_login": event.OwnerLogin,
		"image":       event.Image,
		"node_id":     event.AgentID,
		"ports":       event.Ports,
	}

	switch event.Type {
	case "deploy.completed", "container.restarted":
		payload["status"] = "running"
	case "deploy.failed", "op.failed":
		payload["status"] = "error"
		payload["error"] = event.Message
	case "container.stopped":
		payload["status"] = "stopped"
	case "container.removed":
		payload["removed"] = true
	default:
		return
	}

	hub.Emit("container.updated", realtime.TopicContainers, event.OwnerLogin, payload)
}

// releaseOpForTerminal снимает блокировку операции, когда пришло терминальное событие.
// У сервис-контейнеров compose замок может висеть и на родителе, и на самом
// сервисе: обновление сервиса блокируется по его app_name, а события деплоя
// приходят с родительским OpApp. Снимаем оба ключа, иначе дочерний замок
// висел бы до TTL.
func releaseOpForTerminal(event admin.AgentEvent, containersService *containers.Service) {
	switch event.Type {
	case "deploy.completed", "deploy.failed", "op.failed",
		"container.stopped", "container.restarted", "container.removed":
		containersService.ReleaseOp(event.OwnerLogin, event.OpApp())
		if event.AppName != "" && event.AppName != event.OpApp() {
			containersService.ReleaseOp(event.OwnerLogin, event.AppName)
		}
	}
}
