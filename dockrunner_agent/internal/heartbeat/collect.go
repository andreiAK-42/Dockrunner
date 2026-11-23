package heartbeat

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"dockrunner_agent/internal/config"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/fsx"
	"dockrunner_agent/internal/geo"
	"dockrunner_agent/internal/store"
)

func toGB(bytes float64) float64 {
	return bytes / 1024 / 1024 / 1024
}

func storageEntry(byOwner map[string]*UserStorage, owner string) *UserStorage {
	entry, ok := byOwner[owner]
	if !ok {
		entry = &UserStorage{Owner: owner}
		byOwner[owner] = entry
	}
	return entry
}

func collectStorage(ctx context.Context, db *sql.DB, dock *docker.Client, workspace string) []UserStorage {
	byOwner := map[string]*UserStorage{}

	deployments, err := store.ListDeployments(db)
	if err == nil {
		sizes, _ := dock.ContainerSizes(ctx)
		for _, dep := range deployments {
			if dep.Owner == "" || dep.Container == "" {
				continue
			}
			entry := storageEntry(byOwner, dep.Owner)
			entry.ContainersGB += toGB(float64(sizes[dep.Container]))
		}
	}

	items, err := os.ReadDir(workspace)
	if err == nil {
		for _, item := range items {
			if !item.IsDir() {
				continue
			}
			owner := item.Name()
			entry := storageEntry(byOwner, owner)
			size, sizeErr := fsx.DirSize(workspace + "/" + owner)
			if sizeErr == nil {
				entry.FilesGB = toGB(float64(size))
			}
		}
	}

	out := make([]UserStorage, 0, len(byOwner))
	for _, entry := range byOwner {
		out = append(out, *entry)
	}
	return out
}

func Collect(ctx context.Context, db *sql.DB, dock *docker.Client, cfg *config.Config, geo *geo.Location) (Report, error) {
	report := Report{
		AgentID:     cfg.AgentName,
		AgentName:   cfg.AgentName,
		IP:          cfg.AgentIP,
		Description: cfg.AgentDescription,
		City:        geo.City,
		Lat:         geo.Lat,
		Lng:         geo.Lng,
		ApiURL:      cfg.AgentPublicURL,
	}

	percents, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err == nil && len(percents) > 0 {
		report.CPUPercent = percents[0]
	}

	total, err := cpu.CountsWithContext(ctx, true)
	if err == nil {
		report.CPUTotal = total
	}

	cpuInfo, err := cpu.InfoWithContext(ctx)
	if err == nil && len(cpuInfo) > 0 {
		report.CPUMhz = int(cpuInfo[0].Mhz)
	}

	memInfo, err := mem.VirtualMemoryWithContext(ctx)
	if err == nil {
		report.MemUsedGB = toGB(float64(memInfo.Used))
		report.MemTotalGB = toGB(float64(memInfo.Total))
		report.MemSpeedMhz = detectMemSpeed()
	}

	root := rootMountPoint()
	diskInfo, err := disk.UsageWithContext(ctx, root)
	if err == nil {
		report.DiskUsedGB = toGB(float64(diskInfo.Used))
		report.DiskTotalGB = toGB(float64(diskInfo.Total))
	} else if runtime.GOOS == "windows" {
		// На Windows корень может быть недоступен — берём первый раздел.
		parts, partsErr := disk.Partitions(false)
		if partsErr == nil {
			for _, part := range parts {
				usage, usageErr := disk.UsageWithContext(ctx, part.Mountpoint)
				if usageErr != nil {
					continue
				}
				report.DiskUsedGB = toGB(float64(usage.Used))
				report.DiskTotalGB = toGB(float64(usage.Total))
				break
			}
		}
	}

	ids, err := dock.RunningIDs(ctx)
	if err == nil {
		report.Containers = len(ids)
	}

	report.Storage = collectStorage(ctx, db, dock, cfg.Workspace)
	report.NetDownMbps, report.NetUpMbps = netSpeed()
	return report, nil
}
