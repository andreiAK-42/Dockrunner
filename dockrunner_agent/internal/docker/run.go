package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func (c *Client) Run(ctx context.Context, spec RunSpec) (string, error) {
	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}
	exposed := nat.PortSet{}
	bindings := nat.PortMap{}
	for _, p := range spec.Ports {
		port, err := nat.NewPort("tcp", fmt.Sprint(p.Container))
		if err != nil {
			return "", err
		}
		exposed[port] = struct{}{}
		bindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprint(p.Host)}}
	}
	binds := make([]string, 0, len(spec.Volumes))
	for _, v := range spec.Volumes {
		binds = append(binds, v.Name+":"+v.Path)
	}
	policy := spec.RestartPolicy
	if policy == "" {
		policy = "unless-stopped"
	}
	resources := container.Resources{}
	if spec.CPUCores > 0 {
		resources.NanoCPUs = int64(spec.CPUCores * 1e9)
	}
	if spec.MemoryGB > 0 {
		resources.Memory = int64(spec.MemoryGB * 1024 * 1024 * 1024)
	}
	// StorageOpt для ограничения размера writable layer контейнера (overlay2.size)
	storageOpt := map[string]string{}
	if spec.DiskGB > 0 {
		storageOpt["size"] = fmt.Sprintf("%.0fG", spec.DiskGB)
	}
	labels := map[string]string{}
	if spec.DeploymentID != "" {
		labels[LabelDeployment] = spec.DeploymentID
	}
	if spec.AppName != "" {
		labels[LabelApp] = spec.AppName
	}
	if spec.Owner != "" {
		labels[LabelOwner] = spec.Owner
	}
	netConfig := &network.NetworkingConfig{}
	hostConfig := &container.HostConfig{
		PortBindings:  bindings,
		Binds:         binds,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyMode(policy)},
		Resources:     resources,
		StorageOpt:    storageOpt,
	}
	if spec.Network != "" {
		hostConfig.NetworkMode = container.NetworkMode(spec.Network)
		netConfig.EndpointsConfig = map[string]*network.EndpointSettings{
			spec.Network: {},
		}
	}
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{Image: spec.Image, Env: env, ExposedPorts: exposed, Entrypoint: spec.Entrypoint, Cmd: spec.Cmd, Labels: labels},
		hostConfig,
		netConfig,
		&ocispec.Platform{},
		spec.Name,
	)
	if err != nil && len(storageOpt) > 0 && strings.Contains(err.Error(), "storage-opt is supported only") {
		// Демон на ext4 или xfs без pquota — квота размера недоступна.
		// Запускаем без лимита диска, чтобы не ронять весь деплой.
		hostConfig.StorageOpt = map[string]string{}
		resp, err = c.cli.ContainerCreate(ctx,
			&container.Config{Image: spec.Image, Env: env, ExposedPorts: exposed, Entrypoint: spec.Entrypoint, Cmd: spec.Cmd, Labels: labels},
			hostConfig,
			netConfig,
			&ocispec.Platform{},
			spec.Name,
		)
	}
	if err != nil {
		return "", err
	}
	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}
	return resp.ID, nil
}
