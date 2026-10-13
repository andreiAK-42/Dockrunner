package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func run(ctx context.Context, dir, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

func EnsureCloned(ctx context.Context, repoURL, dir string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git not installed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return syncExisting(ctx, repoURL, dir)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	return run(ctx, "", "git", "clone", "--depth", "1", repoURL, dir)
}

func syncExisting(ctx context.Context, repoURL, dir string) error {
	if err := run(ctx, dir, "git", "remote", "set-url", "origin", repoURL); err != nil {
		return err
	}
	if err := run(ctx, dir, "git", "fetch", "--depth", "1", "origin", "HEAD"); err != nil {
		return err
	}
	return run(ctx, dir, "git", "reset", "--hard", "FETCH_HEAD")
}

func ListRoot(dir string) []string {
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name()
		if item.IsDir() {
			name += "/"
		}
		names = append(names, name)
		if len(names) >= 20 {
			break
		}
	}
	return names
}

func Detect(dir string) (dockerfile string, compose string) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}
	for _, item := range items {
		if item.IsDir() {
			continue
		}
		lower := strings.ToLower(item.Name())
		if lower == "dockerfile" && dockerfile == "" {
			dockerfile = item.Name()
			continue
		}
		if compose == "" && isComposeFile(lower) {
			compose = item.Name()
		}
	}
	return dockerfile, compose
}

func isComposeFile(lower string) bool {
	switch lower {
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		return true
	}
	return false
}
