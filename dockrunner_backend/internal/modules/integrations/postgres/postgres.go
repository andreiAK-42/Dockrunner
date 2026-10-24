package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"dockrunner_backend/internal/modules/integrations/engine"
)

type Manager struct {
	AdminURL string
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func (m Manager) connect(ctx context.Context) (*pgx.Conn, error) {
	if m.AdminURL == "" {
		return nil, engine.ErrUnreachable
	}
	conn, err := pgx.Connect(ctx, m.AdminURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", engine.ErrUnreachable, err)
	}
	return conn, nil
}

func (m Manager) CreateUser(ctx context.Context, dbName, username, password string) error {
	conn, err := m.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, `CREATE ROLE `+quoteIdent(username)+` WITH LOGIN PASSWORD `+quoteLiteral(password)); err != nil {
		return fmt.Errorf("create role: %w", err)
	}
	if _, err := conn.Exec(ctx, `CREATE DATABASE `+quoteIdent(dbName)+` OWNER `+quoteIdent(username)); err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	if _, err := conn.Exec(ctx, `GRANT ALL PRIVILEGES ON DATABASE `+quoteIdent(dbName)+` TO `+quoteIdent(username)); err != nil {
		return fmt.Errorf("grant: %w", err)
	}
	return nil
}

func (m Manager) DropUser(ctx context.Context, dbName, username string) error {
	conn, err := m.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, _ = conn.Exec(ctx, `DROP DATABASE IF EXISTS `+quoteIdent(dbName))
	if _, err := conn.Exec(ctx, `DROP ROLE IF EXISTS `+quoteIdent(username)); err != nil {
		return fmt.Errorf("drop role: %w", err)
	}
	return nil
}
