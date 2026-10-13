package files

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"dockrunner_backend/pkg/queue"
)

const (
	rpcTimeout = 30 * time.Second

	// downloadLimit — защита от выкачивания огромных файлов в память.
	downloadLimit = 50 * 1024 * 1024
)

type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

type Service struct {
	queue  *queue.Client
	logger *zap.Logger
}

func NewService(queue *queue.Client, logger *zap.Logger) *Service {
	return &Service{queue: queue, logger: logger}
}

type fileResponse struct {
	OK         bool    `json:"ok"`
	Error      string  `json:"error"`
	Entries    []Entry `json:"entries"`
	ContentB64 string  `json:"content_b64"`
}

func (s *Service) call(ctx context.Context, node, taskType, owner, path, contentB64 string) (*fileResponse, error) {
	if s.queue == nil {
		return nil, errors.New("queue unavailable")
	}
	body, _ := json.Marshal(map[string]string{
		"type":        taskType,
		"owner":       owner,
		"path":        path,
		"content_b64": contentB64,
	})
	raw, err := s.queue.RpcCall(ctx, "agent."+node, body, rpcTimeout)
	if err != nil {
		return nil, err
	}
	var res fileResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	if !res.OK {
		if res.Error == "" {
			res.Error = "agent error"
		}
		return nil, errors.New(res.Error)
	}
	return &res, nil
}

func (s *Service) List(ctx context.Context, owner, node, path string) ([]Entry, error) {
	res, err := s.call(ctx, node, "files.list", owner, path, "")
	if err != nil {
		return nil, err
	}
	if res.Entries == nil {
		return []Entry{}, nil
	}
	return res.Entries, nil
}

func (s *Service) Mkdir(ctx context.Context, owner, node, path string) error {
	_, err := s.call(ctx, node, "files.mkdir", owner, path, "")
	return err
}

func (s *Service) Upload(ctx context.Context, owner, node, path string, data []byte) error {
	_, err := s.call(ctx, node, "files.upload", owner, path, base64.StdEncoding.EncodeToString(data))
	return err
}

func (s *Service) Download(ctx context.Context, owner, node, path string) ([]byte, error) {
	res, err := s.call(ctx, node, "files.download", owner, path, "")
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(res.ContentB64)
	if err != nil {
		return nil, err
	}
	if len(data) > downloadLimit {
		return nil, errors.New("file too large")
	}
	return data, nil
}

func (s *Service) Delete(ctx context.Context, owner, node, path string) error {
	_, err := s.call(ctx, node, "files.delete", owner, path, "")
	return err
}
