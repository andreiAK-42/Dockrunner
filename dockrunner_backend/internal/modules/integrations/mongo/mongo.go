package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"dockrunner_backend/internal/modules/integrations/engine"
)

type Manager struct {
	AdminURL string
}

func (m Manager) client(ctx context.Context) (*mongodriver.Client, error) {
	if m.AdminURL == "" {
		return nil, engine.ErrUnreachable
	}
	client, err := mongodriver.Connect(options.Client().ApplyURI(m.AdminURL))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", engine.ErrUnreachable, err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("%w: %v", engine.ErrUnreachable, err)
	}
	return client, nil
}

func (m Manager) CreateUser(ctx context.Context, dbName, username, password string) error {
	client, err := m.client(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(ctx) }()
	res := client.Database(dbName).RunCommand(ctx, bson.D{
		{Key: "createUser", Value: username},
		{Key: "pwd", Value: password},
		{Key: "roles", Value: bson.A{bson.D{{Key: "role", Value: "readWrite"}, {Key: "db", Value: dbName}}}},
	})
	return res.Err()
}

func (m Manager) DropUser(ctx context.Context, dbName, username string) error {
	client, err := m.client(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(ctx) }()
	res := client.Database(dbName).RunCommand(ctx, bson.D{{Key: "dropUser", Value: username}})
	return res.Err()
}
