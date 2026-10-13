package queue

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"go.uber.org/zap"
)

type DeployTask struct {
	AppID  string `json:"app_id"`
	Image  string `json:"image"`
	NodeID string `json:"node_id"`
}

type Notice struct {
	UserID string `json:"user_id"`
	Title  string `json:"title"`
	Text   string `json:"text"`
}

func Example_publishdeploy() {
	if os.Getenv("RABBITMQ_URL") == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Dial(ctx, Config{URL: os.Getenv("RABBITMQ_URL")}, zap.NewNop())
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()
	if err := client.DeclareTopology(ctx); err != nil {
		return
	}

	body, _ := json.Marshal(DeployTask{AppID: "app-01", Image: "nginx:1.27", NodeID: "worker-01"})
	_ = client.Publish(ctx, QueueDeploys, body)
}

func Example_consumenotifications() {
	if os.Getenv("RABBITMQ_URL") == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := Dial(ctx, Config{URL: os.Getenv("RABBITMQ_URL")}, zap.NewNop())
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()
	if err := client.DeclareTopology(ctx); err != nil {
		return
	}

	_ = client.Consume(QueueNotifications, func(ctx context.Context, body []byte) error {
		var notice Notice
		return json.Unmarshal(body, &notice)
	})
}
