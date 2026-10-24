package redis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"dockrunner_backend/internal/modules/integrations/engine"
)

type Manager struct {
	Addr     string
	Password string
}

func (m Manager) dial() (net.Conn, *bufio.Reader, error) {
	if m.Addr == "" {
		return nil, nil, engine.ErrUnreachable
	}
	conn, err := net.DialTimeout("tcp", m.Addr, 5*time.Second)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", engine.ErrUnreachable, err)
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

func (m Manager) auth(conn net.Conn, r *bufio.Reader) error {
	if m.Password == "" {
		return nil
	}
	if err := respCall(conn, r, "AUTH", m.Password); err != nil {
		return fmt.Errorf("redis auth: %w", err)
	}
	return nil
}

func (m Manager) CreateUser(_ context.Context, _, username, password string) error {
	conn, r, err := m.dial()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := m.auth(conn, r); err != nil {
		return err
	}
	return respCall(conn, r, "ACL", "SETUSER", username, "on", ">"+password, "~*", "+@all")
}

func (m Manager) DropUser(_ context.Context, _, username string) error {
	conn, r, err := m.dial()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if err := m.auth(conn, r); err != nil {
		return err
	}
	return respCall(conn, r, "ACL", "DELUSER", username)
}
