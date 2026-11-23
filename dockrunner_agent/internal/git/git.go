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

func EnsureCloned(ctx context.Context, repoURL, dir, branch string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git not installed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return syncExisting(ctx, repoURL, dir, branch)
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	args := []string{"clone", "--depth", "1"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, repoURL, dir)
	return run(ctx, "", "git", args...)
}

func syncExisting(ctx context.Context, repoURL, dir, branch string) error {
	if err := run(ctx, dir, "git", "remote", "set-url", "origin", repoURL); err != nil {
		return err
	}
	ref := "HEAD"
	if branch != "" {
		ref = branch
	}
	if err := run(ctx, dir, "git", "fetch", "--depth", "1", "origin", ref); err != nil {
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

// ListRemoteBranches lists all branches from a remote repository without cloning
func ListRemoteBranches(ctx context.Context, repoURL string) ([]string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("git not installed")
	}
	// Use git ls-remote to get branches without cloning
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", repoURL)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	var branches []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: <commit-hash>\trefs/heads/<branch-name>
		parts := strings.Split(line, "\t")
		if len(parts) == 2 && strings.HasPrefix(parts[1], "refs/heads/") {
			branchName := strings.TrimPrefix(parts[1], "refs/heads/")
			branches = append(branches, branchName)
		}
	}
	return branches, nil
}

// ListLocalBranches lists local branches in a repository.
// Remote-tracking refs (origin/main, origin/HEAD и т.п.) не возвращаем:
// это не ветки, и git fetch/clone --branch с ними падает
// ("couldn't find remote ref origin/main").
func ListLocalBranches(dir string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.New("git not installed")
	}
	cmd := exec.CommandContext(ctx, "git", "branch", "--format=%(refname:short)")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	var branches []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !seen[line] {
			branches = append(branches, line)
			seen[line] = true
		}
	}
	return branches, nil
}
