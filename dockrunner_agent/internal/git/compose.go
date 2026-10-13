package git

import (
	"errors"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

var errNoServices = errors.New("no services in compose file")

type ComposeService struct {
	Name       string
	Image      string
	Dockerfile string
	ContextDir string
}

type composeServiceRaw struct {
	Image string `yaml:"image"`
	Build any    `yaml:"build"`
}

type composeFileRaw struct {
	Services map[string]composeServiceRaw `yaml:"services"`
}

// ParseComposeServices разбирает ВСЕ сервисы compose-файла (отсортированы по имени).
func ParseComposeServices(dir, file string) ([]ComposeService, error) {
	raw, err := os.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return nil, err
	}
	var parsed composeFileRaw
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(parsed.Services))
	for name := range parsed.Services {
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, errNoServices
	}
	sort.Strings(names)
	out := make([]ComposeService, 0, len(names))
	for _, name := range names {
		svc := parsed.Services[name]
		cs := ComposeService{Name: name, Image: svc.Image, Dockerfile: "Dockerfile", ContextDir: dir}
		switch build := svc.Build.(type) {
		case string:
			cs.ContextDir = filepath.Join(dir, build)
		case map[string]any:
			if contextDir, ok := build["context"].(string); ok && contextDir != "" {
				cs.ContextDir = filepath.Join(dir, contextDir)
			}
			if dockerfile, ok := build["dockerfile"].(string); ok && dockerfile != "" {
				cs.Dockerfile = dockerfile
			}
		default:
			cs.Dockerfile = ""
		}
		out = append(out, cs)
	}
	return out, nil
}

// ParseComposeService — совместимость: первый сервис (сортировка по имени).
func ParseComposeService(dir, file string) (ComposeService, error) {
	all, err := ParseComposeServices(dir, file)
	if err != nil {
		return ComposeService{}, err
	}
	return all[0], nil
}

// ParseComposeNames возвращает только имена сервисов (для prepare-ответа).
func ParseComposeNames(dir, file string) ([]string, error) {
	all, err := ParseComposeServices(dir, file)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(all))
	for _, svc := range all {
		names = append(names, svc.Name)
	}
	return names, nil
}
