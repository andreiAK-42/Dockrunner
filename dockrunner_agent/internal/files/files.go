package files

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"dockrunner_agent/internal/fsx"
)

type Entry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

func resolveDir(workspace, owner, input string) (string, error) {
	abs, err := fsx.ResolveHostPath(workspace, owner, input)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("not a directory")
	}
	return abs, nil
}

func List(workspace, owner, input string) ([]Entry, error) {
	dir := input
	if dir == "" {
		dir = "/"
	}
	abs, err := resolveDir(workspace, owner, dir)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(items))
	for _, item := range items {
		entry := Entry{Name: item.Name(), IsDir: item.IsDir()}
		if !item.IsDir() {
			if info, err := item.Info(); err == nil {
				entry.Size = info.Size()
			}
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// validUserPath — путь внутри папки пользователя; корень запрещён.
func validUserPath(input string) bool {
	return input != "" && input != "/"
}

func Mkdir(workspace, owner, input string) error {
	if !validUserPath(input) {
		return errors.New("bad path")
	}
	abs, err := fsx.ResolveHostPath(workspace, owner, input)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

func resolveFile(workspace, owner, input string) (string, error) {
	if !validUserPath(input) {
		return "", errors.New("bad path")
	}
	return fsx.Jail(workspace, owner, input)
}

func Write(workspace, owner, input string, data []byte) error {
	abs, err := resolveFile(workspace, owner, input)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0o644)
}

func Delete(workspace, owner, input string) error {
	abs, err := resolveFile(workspace, owner, input)
	if err != nil {
		return err
	}
	return os.RemoveAll(abs)
}

func Read(workspace, owner, input string) ([]byte, error) {
	abs, err := resolveFile(workspace, owner, input)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, errors.New("not a file")
	}
	return os.ReadFile(abs)
}
