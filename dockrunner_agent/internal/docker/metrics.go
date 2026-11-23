package docker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func (c *Client) ContainerStatus(ctx context.Context, id string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}
	return info.State.Status, nil
}

func (c *Client) RunningIDs(ctx context.Context) ([]string, error) {
	list, err := c.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		if item.State == "running" {
			ids = append(ids, item.ID)
		}
	}
	return ids, nil
}

func (c *Client) ContainerSizes(ctx context.Context) (map[string]int64, error) {
	usage, err := c.cli.DiskUsage(ctx, types.DiskUsageOptions{Types: []types.DiskUsageObject{types.ContainerObject}})
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(usage.Containers))
	for _, ctr := range usage.Containers {
		out[ctr.ID] = ctr.SizeRw
	}
	return out, nil
}

func readOneStat(ctx context.Context, cli *client.Client, id string) (container.StatsResponse, error) {
	res, err := cli.ContainerStats(ctx, id, false)
	if err != nil {
		return container.StatsResponse{}, err
	}
	defer func() { _ = res.Body.Close() }()
	var st container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&st); err != nil {
		return container.StatsResponse{}, err
	}
	return st, nil
}

func (c *Client) Stats(ctx context.Context, id string) (Stats, error) {
	first, err := readOneStat(ctx, c.cli, id)
	if err != nil {
		return Stats{}, err
	}
	select {
	case <-ctx.Done():
		return Stats{}, ctx.Err()
	case <-time.After(time.Second):
	}
	second, err := readOneStat(ctx, c.cli, id)
	if err != nil {
		return Stats{}, err
	}
	cpuDelta := float64(second.CPUStats.CPUUsage.TotalUsage - first.CPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(second.CPUStats.SystemUsage - first.CPUStats.SystemUsage)
	cpu := 0.0
	if sysDelta > 0 {
		cpu = cpuDelta / sysDelta * float64(cpuCount(second)) * 100
	}
	return Stats{CPUPercent: cpu, MemMB: float64(second.MemoryStats.Usage) / 1024 / 1024}, nil
}

// cpuCount — число CPU по статистике: сначала OnlineCPUs,
// потом длина PercpuUsage, иначе 1 (деление на ноль исключено).
func cpuCount(st container.StatsResponse) int {
	if st.CPUStats.OnlineCPUs != 0 {
		return int(st.CPUStats.OnlineCPUs)
	}
	if len(st.CPUStats.CPUUsage.PercpuUsage) != 0 {
		return len(st.CPUStats.CPUUsage.PercpuUsage)
	}
	return 1
}
