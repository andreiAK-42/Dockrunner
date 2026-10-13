package databases

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrUnreachable = errors.New("СУБД недоступна — проверьте host/port/admin-доступ в .env")

// Provider создаёт и удаляет пользователей в живой СУБД.
type Provider interface {
	CreateUser(ctx context.Context, dbName, username, password string) error
	DropUser(ctx context.Context, dbName, username string) error
}

// ---------- PostgreSQL (pgx) ----------

type postgresProvider struct {
	adminURL string
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func quoteLiteral(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

func (p postgresProvider) connect(ctx context.Context) (*pgx.Conn, error) {
	if p.adminURL == "" {
		return nil, ErrUnreachable
	}
	conn, err := pgx.Connect(ctx, p.adminURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return conn, nil
}

func (p postgresProvider) CreateUser(ctx context.Context, dbName, username, password string) error {
	conn, err := p.connect(ctx)
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

func (p postgresProvider) DropUser(ctx context.Context, dbName, username string) error {
	conn, err := p.connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	// Сначала база (роль владеет ей — иначе DROP ROLE упадёт).
	_, _ = conn.Exec(ctx, `DROP DATABASE IF EXISTS `+quoteIdent(dbName))
	if _, err := conn.Exec(ctx, `DROP ROLE IF EXISTS `+quoteIdent(username)); err != nil {
		return fmt.Errorf("drop role: %w", err)
	}
	return nil
}

// ---------- MongoDB (mongo-driver) ----------

type mongoProvider struct {
	adminURL string
}

func (p mongoProvider) client(ctx context.Context) (*mongo.Client, error) {
	if p.adminURL == "" {
		return nil, ErrUnreachable
	}
	client, err := mongo.Connect(options.Client().ApplyURI(p.adminURL))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return client, nil
}

func (p mongoProvider) CreateUser(ctx context.Context, dbName, username, password string) error {
	client, err := p.client(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(ctx) }()
	res := client.Database(dbName).RunCommand(ctx, bson.D{
		{"createUser", username},
		{"pwd", password},
		{"roles", bson.A{bson.D{{"role", "readWrite"}, {"db", dbName}}}},
	})
	return res.Err()
}

func (p mongoProvider) DropUser(ctx context.Context, dbName, username string) error {
	client, err := p.client(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = client.Disconnect(ctx) }()
	res := client.Database(dbName).RunCommand(ctx, bson.D{{"dropUser", username}})
	return res.Err()
}

// ---------- Redis (минимальный RESP-клиент, без новых зависимостей) ----------

type redisProvider struct {
	addr     string
	password string
}

func (p redisProvider) dial() (net.Conn, *bufio.Reader, error) {
	if p.addr == "" {
		return nil, nil, ErrUnreachable
	}
	conn, err := net.DialTimeout("tcp", p.addr, 5*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	return conn, bufio.NewReader(conn), nil
}

func respCommand(args ...string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(a), a)
	}
	return []byte(b.String())
}

func respCall(conn net.Conn, r *bufio.Reader, args ...string) error {
	if _, err := conn.Write(respCommand(args...)); err != nil {
		return err
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	switch {
	case strings.HasPrefix(line, "+"):
		return nil
	case strings.HasPrefix(line, "-"):
		return errors.New(strings.TrimSpace(line[1:]))
	default:
		return fmt.Errorf("unexpected redis reply: %s", strings.TrimSpace(line))
	}
}

func (p redisProvider) CreateUser(_ context.Context, _, username, password string) error {
	conn, r, err := p.dial()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if p.password != "" {
		if err := respCall(conn, r, "AUTH", p.password); err != nil {
			return fmt.Errorf("redis auth: %w", err)
		}
	}
	// ACL: включён, доступ ко всем ключам/командам (Redis 6+).
	return respCall(conn, r, "ACL", "SETUSER", username, "on", ">"+password, "~*", "+@all")
}

func (p redisProvider) DropUser(_ context.Context, _, username string) error {
	conn, r, err := p.dial()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if p.password != "" {
		if err := respCall(conn, r, "AUTH", p.password); err != nil {
			return fmt.Errorf("redis auth: %w", err)
		}
	}
	return respCall(conn, r, "ACL", "DELUSER", username)
}
