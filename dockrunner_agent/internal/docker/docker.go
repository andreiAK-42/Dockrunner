package docker

import (
	"context"
	"io"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type PortMapping struct {
	Host      int
	Container int
}

type VolumeMount struct {
	Name string
	Path string
}

type RunSpec struct {
	Name          string
	Image         string
	Env           map[string]string
	Ports         []PortMapping
	Volumes       []VolumeMount
	RestartPolicy string
	Entrypoint    []string
	Cmd           []string
	// CPUCores — лимит CPU на контейнер (0 = без лимита).
	CPUCores float64
	// MemoryGB — лимит RAM на контейнер в ГБ (0 = без лимита).
	MemoryGB float64
	// DiskGB — лимит диска на контейнер в ГБ (0 = без лимита).
	// Применяется через --storage-opt size=X для overlay2 (ограничивает writable layer).
	DiskGB float64
	// Network — пользовательская docker-сеть ("" = default).
	Network string
	// DeploymentID/AppName/Owner попадают в docker-метки: по ним агент
	// находит свой контейнер, если локальная SQLite потерялась.
	DeploymentID string
	AppName      string
	Owner        string
}

const (
	// LabelDeployment/LabelApp/LabelOwner — метки, по которым
	// восстанавливается потерянная запись о деплое (SQLite агента
	// пересоздали без volume). Owner нужен, чтобы не перепутать
	// одноимённые приложения разных пользователей.
	LabelDeployment = "dockrunner.deployment"
	LabelApp        = "dockrunner.app"
	LabelOwner      = "dockrunner.owner"
)

type Stats struct {
	CPUPercent float64
	MemMB      float64
}

type Client struct {
	cli *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Close() error {
	return c.cli.Close()
}

func (c *Client) NetworkList(ctx context.Context, opts network.ListOptions) ([]network.Summary, error) {
	return c.cli.NetworkList(ctx, opts)
}

func IsMissing(err error) bool {
	return err != nil && errdefs.IsNotFound(err)
}

func (c *Client) EnsureImage(ctx context.Context, ref string) error {
	rc, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, rc)
	return rc.Close()
}

func mapToEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}

// EnsureNetwork создаёт пользовательскую сеть, если её ещё нет.
// Все сервисы одного compose-проекта садятся в неё и видят друг друга по именам.
func (c *Client) EnsureNetwork(ctx context.Context, name string) error {
	if _, err := c.cli.NetworkInspect(ctx, name, network.InspectOptions{}); err == nil {
		return nil
	}
	_, err := c.cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"})
	return err
}

func (c *Client) Stop(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) Restart(ctx context.Context, id string) error {
	timeout := 10
	return c.cli.ContainerRestart(ctx, id, container.StopOptions{Timeout: &timeout})
}

func (c *Client) Remove(ctx context.Context, id string) error {
	return c.cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: false})
}

// Rename переименовывает контейнер (например, чтобы сохранить старую версию
// как резерв до успешного запуска новой).
func (c *Client) Rename(ctx context.Context, id, name string) error {
	return c.cli.ContainerRename(ctx, id, name)
}

// Start запускает ранее остановленный контейнер.
func (c *Client) Start(ctx context.Context, id string) error {
	return c.cli.ContainerStart(ctx, id, container.StartOptions{})
}

func (c *Client) VolumeRemove(ctx context.Context, name string) error {
	return c.cli.VolumeRemove(ctx, name, false)
}
