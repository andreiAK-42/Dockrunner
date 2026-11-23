package docker

import (
	"context"
	"io"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

func (c *Client) LogsTail(ctx context.Context, id, tail string) (string, error) {
	// Контейнеры с TTY (например, созданные вручную в Portainer) отдают
	// не мультиплексированный поток: stdcopy на нём падает и лог получается
	// пустым, хотя docker logs его показывает.
	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return "", err
	}
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: false, Tail: tail,
	})
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	var out, errOut lineBuffer
	if tty {
		_, _ = io.Copy(&out, rc)
	} else {
		_, _ = stdcopy.StdCopy(&out, &errOut, rc)
	}
	out.Flush()
	errOut.Flush()
	return out.String() + errOut.String(), nil
}

// containerTTY сообщает, запущен ли контейнер с TTY.
func (c *Client) containerTTY(ctx context.Context, id string) (bool, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return false, err
	}
	return info.Config != nil && info.Config.Tty, nil
}

func (c *Client) FollowLogs(ctx context.Context, id string, emit func(stream, line string)) error {
	tty, err := c.containerTTY(ctx, id)
	if err != nil {
		return err
	}
	rc, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true, Tail: "50",
	})
	if err != nil {
		return err
	}
	defer func() { _ = rc.Close() }()
	go func() {
		<-ctx.Done()
		_ = rc.Close()
	}()
	out := &lineWriter{emit: func(line string) { emit("stdout", line) }}
	errOut := &lineWriter{emit: func(line string) { emit("stderr", line) }}
	if tty {
		_, _ = io.Copy(out, rc)
	} else {
		_, _ = stdcopy.StdCopy(out, errOut, rc)
	}
	out.Flush()
	errOut.Flush()
	return nil
}

// splitLines достаёт из буфера законченные строки и отдаёт их в emit.
func splitLines(buf *strings.Builder, emit func(line string)) {
	for {
		s := buf.String()
		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			break
		}
		emit(s[:idx])
		buf.Reset()
		buf.WriteString(s[idx+1:])
	}
}

type lineBuffer struct {
	lines []string
	buf   strings.Builder
}

func (w *lineBuffer) Write(p []byte) (int, error) {
	w.buf.Write(p)
	splitLines(&w.buf, func(line string) {
		w.lines = append(w.lines, line)
	})
	return len(p), nil
}

func (w *lineBuffer) Flush() {
	if rest := w.buf.String(); rest != "" {
		w.lines = append(w.lines, rest)
		w.buf.Reset()
	}
}

func (w *lineBuffer) String() string {
	return strings.Join(w.lines, "\n")
}

type lineWriter struct {
	buf  strings.Builder
	emit func(line string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	splitLines(&w.buf, w.emit)
	return len(p), nil
}

func (w *lineWriter) Flush() {
	if rest := strings.TrimRight(w.buf.String(), "\r\n"); rest != "" {
		w.emit(rest)
	}
	w.buf.Reset()
}
