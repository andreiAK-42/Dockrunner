package volumes

import (
	"context"
	"encoding/json"
	"errors"

	"go.uber.org/zap"

	"dockrunner_backend/pkg/queue"
)

var ErrNoQueue = errors.New("queue unavailable")

type Service struct {
	repo   *Repository
	queue  *queue.Client
	logger *zap.Logger
}

func NewService(repo *Repository, queue *queue.Client, logger *zap.Logger) *Service {
	return &Service{repo: repo, queue: queue, logger: logger}
}

func (s *Service) List(ctx context.Context, owner string) ([]Volume, error) {
	return s.repo.ListByOwner(ctx, owner)
}

func (s *Service) Create(ctx context.Context, owner, node, name string, sizeGB float64, containerPath string) (*Volume, error) {
	volume := Volume{NodeID: node, Name: name, OwnerLogin: owner, ContainerPath: containerPath, SizeGB: sizeGB, Status: "available"}
	if err := s.repo.Upsert(ctx, volume); err != nil {
		return nil, err
	}
	return &volume, nil
}

func (s *Service) Delete(ctx context.Context, owner, node, name string) error {
	if s.queue == nil {
		return ErrNoQueue
	}
	body, _ := json.Marshal(map[string]string{"type": "volume.remove", "name": name})
	if err := s.queue.Publish(ctx, "agent."+node, body); err != nil {
		return err
	}
	return s.repo.Delete(ctx, owner, node, name)
}

type NamedVolume struct {
	Name          string
	ContainerPath string
}

func (s *Service) TrackFromRun(ctx context.Context, owner, node string, volumes []NamedVolume) {
	for _, v := range volumes {
		if v.Name == "" {
			continue
		}
		_ = s.repo.Upsert(ctx, Volume{
			NodeID: node, Name: v.Name, OwnerLogin: owner,
			ContainerPath: v.ContainerPath, Status: "attached",
		})
	}
}
