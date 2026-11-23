package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

func sendOnce(ctx context.Context, centralURL, token string, report Report) error {
	body, _ := json.Marshal(report)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, centralURL+"/api/v1/agents/heartbeat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return &statusError{res.StatusCode}
	}
	return nil
}

type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return "heartbeat rejected: " + http.StatusText(e.code)
}
