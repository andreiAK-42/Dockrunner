package ollama

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dockrunner_backend/internal/modules/integrations/engine"
)

type Manager struct {
	BaseURL string
}

func (m Manager) CreateUser(ctx context.Context, _, _, _ string) error {
	return m.ping(ctx)
}

func (m Manager) DropUser(ctx context.Context, _, _ string) error {
	return nil
}

func (m Manager) ping(ctx context.Context) error {
	if m.BaseURL == "" {
		return engine.ErrUnreachable
	}
	url := strings.TrimRight(m.BaseURL, "/") + "/api/tags"
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return engine.ErrUnreachable
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", engine.ErrUnreachable, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return fmt.Errorf("%w: http %d", engine.ErrUnreachable, res.StatusCode)
	}
	return nil
}
