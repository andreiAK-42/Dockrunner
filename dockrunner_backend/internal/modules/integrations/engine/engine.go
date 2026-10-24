package engine

import (
	"context"
	"errors"
)

var ErrUnreachable = errors.New("сервис недоступен — проверьте настройки подключения")

// Manager создаёт и удаляет учётки в живом сервисе (СУБД, Ollama и т.д.).
type Manager interface {
	CreateUser(ctx context.Context, resource, username, password string) error
	DropUser(ctx context.Context, resource, username string) error
}
