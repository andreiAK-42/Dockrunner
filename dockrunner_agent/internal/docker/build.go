package docker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/docker/docker/api/types/build"
)

type buildMessage struct {
	Stream      string `json:"stream"`
	Status      string `json:"status"`
	Progress    string `json:"progress"`
	Error       string `json:"error"`
	ErrorDetail struct {
		Message string `json:"message"`
	} `json:"errorDetail"`
}

func buildErrorText(msg buildMessage) string {
	if msg.ErrorDetail.Message != "" {
		return msg.ErrorDetail.Message
	}
	return msg.Error
}

func statusText(msg buildMessage) string {
	if msg.Progress != "" {
		return msg.Status + " " + msg.Progress
	}
	return msg.Status
}

// buildArgsFromEnv превращает переменные окружения деплоя в build-args.
// Ключи, которые нельзя объявить в Dockerfile через ARG, пропускаем —
// сборка всё равно не смогла бы их использовать, а Docker может упасть
// на недопустимом имени.
func buildArgsFromEnv(env map[string]string) map[string]*string {
	if len(env) == 0 {
		return nil
	}
	args := make(map[string]*string, len(env))
	for k, v := range env {
		if !validBuildArgName(k) {
			continue
		}
		value := v
		args[k] = &value
	}
	if len(args) == 0 {
		return nil
	}
	return args
}

// BuildArgsFromEnv exposes the build arguments Docker actually receives for a
// deployment env, so the UI can show them next to the launch command.
func BuildArgsFromEnv(env map[string]string) map[string]string {
	args := buildArgsFromEnv(env)
	out := make(map[string]string, len(args))
	for name, value := range args {
		if value != nil {
			out[name] = *value
		}
	}
	return out
}

func validBuildArgName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

func (c *Client) Build(ctx context.Context, contextDir, dockerfile, tag string, buildArgs map[string]string, onLine func(line string, progress bool)) error {
	tar := tarDirectory(contextDir)
	resp, err := c.cli.ImageBuild(ctx, tar, build.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{tag},
		BuildArgs:  buildArgsFromEnv(buildArgs),
		Remove:     true,
	})
	_ = tar.Close()
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	emit := func(line string, progress bool) {
		if onLine != nil && line != "" {
			onLine(line, progress)
		}
	}
	var buildErr string
	dec := json.NewDecoder(resp.Body)
	for {
		var msg buildMessage
		if err := dec.Decode(&msg); err != nil {
			break
		}
		switch {
		case msg.Error != "":
			buildErr = buildErrorText(msg)
			emit("ERROR: "+buildErr, false)
		case msg.Stream != "":
			emit(strings.TrimRight(msg.Stream, "\n"), false)
		case msg.Status != "":
			emit(statusText(msg), true)
		}
	}
	if buildErr != "" {
		return errors.New(buildErr)
	}
	return nil
}
