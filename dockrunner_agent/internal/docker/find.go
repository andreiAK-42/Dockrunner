package docker

import (
	"context"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
)

// FindIDByName ищет контейнер по точному имени, включая остановленные.
// Пустая строка означает, что контейнера с таким именем нет.
func (c *Client) FindIDByName(ctx context.Context, name string) (string, error) {
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	if name == "" {
		return "", nil
	}
	info, err := c.cli.ContainerInspect(ctx, name)
	if err != nil {
		if IsMissing(err) {
			return "", nil
		}
		return "", err
	}
	return info.ID, nil
}

// FindIDBySuffix находит контейнер (включая остановленные), имя которого
// заканчивается на suffix. Используется для самовосстановления записей
// о деплоях, потерянных из SQLite (например, после пересоздания контейнера
// агента без volume): имя контейнера держит <app>-<shortID деплоя>.
// Если пользователь задал своё имя контейнера без суффикса, дополнительно
// ищем по тегу образа: агент тегирует собранные образы как app<suffix>:latest.
func (c *Client) FindIDBySuffix(ctx context.Context, suffix string) (string, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	imagePrefix := "app" + suffix + ":"
	stoppedID := ""
	for _, item := range list {
		matched := false
		for _, name := range item.Names {
			if strings.HasSuffix(strings.TrimPrefix(name, "/"), suffix) {
				matched = true
				break
			}
		}
		if !matched && strings.HasPrefix(item.Image, imagePrefix) {
			matched = true
		}
		if !matched {
			continue
		}
		// Приоритет работающему контейнеру: остановленный мог остаться
		// от предыдущей попытки с тем же тегом образа.
		if item.State == "running" {
			return item.ID, nil
		}
		if stoppedID == "" {
			stoppedID = item.ID
		}
	}
	return stoppedID, nil
}

// FindIDByLabels находит контейнер (включая остановленные) по docker-меткам.
// Это основной способ восстановить деплой после потери SQLite: метки
// выставляются при создании контейнера и не зависят от его имени.
func (c *Client) FindIDByLabels(ctx context.Context, labels map[string]string) (string, error) {
	args := make([]filters.KeyValuePair, 0, len(labels))
	for key, value := range labels {
		args = append(args, filters.Arg("label", key+"="+value))
	}
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: filters.NewArgs(args...)})
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "", nil
	}
	return list[0].ID, nil
}

// FindIDByApp находит контейнер приложения, даже если id деплоя уже другой:
// повторный деплой мог не дойти до создания контейнера, а старый контейнер
// приложения продолжает работать. Ищем по метке dockrunner.app, затем по
// точному имени и по имени "<app>-...", отдавая предпочтение запущенным.
// owner (если известен) отсекает чужие контейнеры с тем же именем приложения.
func (c *Client) FindIDByApp(ctx context.Context, appName, owner string) (string, error) {
	appName = strings.TrimSpace(strings.TrimPrefix(appName, "/"))
	if appName == "" {
		return "", nil
	}
	list, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	bestScore := 0
	bestCreated := int64(0)
	bestID := ""
	for _, item := range list {
		if owner != "" {
			if labelOwner := item.Labels[LabelOwner]; labelOwner != "" && labelOwner != owner {
				continue
			}
		}
		score := 0
		if item.Labels[LabelApp] == appName {
			score = 3
		} else {
			for _, raw := range item.Names {
				name := strings.TrimPrefix(raw, "/")
				if name == appName {
					score = 2
					break
				}
				if strings.HasPrefix(name, appName+"-") {
					score = 1
				}
			}
		}
		if score == 0 {
			continue
		}
		if owner != "" && item.Labels[LabelOwner] == owner {
			score += 5
		}
		if item.State == "running" {
			score += 10
		}
		if score > bestScore || (score == bestScore && item.Created > bestCreated) {
			bestScore, bestCreated, bestID = score, item.Created, item.ID
		}
	}
	return bestID, nil
}
