package consumer

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"dockrunner_backend/internal/modules/admin"
	"dockrunner_backend/internal/modules/containers"
	"dockrunner_backend/pkg/queue"
)

func StartEvents(ctx context.Context, q *queue.Client, adminService *admin.Service, containersService *containers.Service, logger *zap.Logger) error {
	return q.Consume(queue.QueueEvents, func(ctx context.Context, body []byte) error {
		var event admin.AgentEvent
		if err := json.Unmarshal(body, &event); err != nil {
			return err
		}

		switch event.Type {
		case "prepare.completed":
			containersService.ResolveCheck(event.DeploymentID, true, event.DetectedDockerfile, event.ComposeFile, event.Services, "")
			logger.Info("prepare resolved", zap.String("deployment", event.DeploymentID), zap.Bool("dockerfile", event.DetectedDockerfile), zap.String("compose", event.ComposeFile), zap.Strings("services", event.Services))
			return nil
		case "prepare.failed":
			containersService.ResolveCheck(event.DeploymentID, false, false, "", nil, event.Message)
			logger.Info("prepare resolved", zap.String("deployment", event.DeploymentID), zap.String("failed", event.Message))
			return nil
		}

		if err := adminService.ApplyEvent(ctx, event); err != nil {
			logger.Warn("apply agent event failed", zap.Error(err))
			return err
		}

		releaseOpForTerminal(event, containersService)
		return nil
	})
}

// releaseOpForTerminal снимает блокировку операции, когда пришло терминальное событие.
// У сервис-контейнеров compose замок висит на родительском приложении.
func releaseOpForTerminal(event admin.AgentEvent, containersService *containers.Service) {
	switch event.Type {
	case "deploy.completed", "deploy.failed", "op.failed",
		"container.stopped", "container.restarted", "container.removed":
		containersService.ReleaseOp(event.OwnerLogin, event.OpApp())
	}
}
