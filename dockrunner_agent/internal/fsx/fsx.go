package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func ProjectDir(base, owner, app string) (string, error) {
	owner = sanitize(owner)
	app = sanitize(app)
	if owner == "" || app == "" {
		return "", errors.New("bad owner or project name")
	}
	return filepath.Join(base, owner, app), nil
}

func UserDir(base, owner string) (string, error) {
	owner = sanitize(owner)
	if owner == "" {
		return "", errors.New("bad owner name")
	}
	return filepath.Join(base, owner), nil
}

func ResolveHostPath(base, owner, input string) (string, error) {
	abs, err := Jail(base, owner, input)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	return abs, nil
}

func Jail(base, owner, input string) (string, error) {
	root, err := UserDir(base, owner)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean("/" + input)
	abs := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || (len(rel) > 3 && rel[:3] == ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes user folder")
	}
	return abs, nil
}

func DirSize(path string) (int64, error) {
	var size int64
	err := filepath.WalkDir(path, func(_ string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size, err
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), ".-")
}
