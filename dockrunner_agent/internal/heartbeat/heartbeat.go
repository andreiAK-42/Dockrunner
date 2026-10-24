package heartbeat

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"

	"dockrunner_agent/internal/config"
	"dockrunner_agent/internal/docker"
	"dockrunner_agent/internal/fsx"
	"dockrunner_agent/internal/store"
)

type UserStorage struct {
	Owner        string  `json:"owner"`
	ContainersGB float64 `json:"containers_gb"`
	FilesGB      float64 `json:"files_gb"`
}

type Report struct {
	AgentID     string        `json:"agent_id"`
	Hostname    string        `json:"hostname"`
	Name        string        `json:"name"`
	IP          string        `json:"ip"`
	Description string        `json:"description"`
	City        string        `json:"city"`
	Lat         float64       `json:"lat"`
	Lng         float64       `json:"lng"`
	ApiURL      string        `json:"api_url"`
	CPUPercent  float64       `json:"cpu_percent"`
	CPUTotal    int           `json:"cpu_total"`
	CPUMhz      int           `json:"cpu_mhz"`
	MemUsedGB   float64       `json:"mem_used_gb"`
	MemTotalGB  float64       `json:"mem_total_gb"`
	MemSpeedMhz int           `json:"mem_speed_mhz"`
	DiskUsedGB  float64       `json:"disk_used_gb"`
	DiskTotalGB float64       `json:"disk_total_gb"`
	Containers  int           `json:"containers"`
	Storage     []UserStorage `json:"storage"`
	NetDownMbps float64       `json:"net_down_mbps"`
	NetUpMbps   float64       `json:"net_up_mbps"`
}

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

func Collect(ctx context.Context, db *sql.DB, dock *docker.Client, cfg *config.Config, hostname string) (Report, error) {
	report := Report{
		AgentID:     cfg.AgentName,
		Hostname:    hostname,
		Name:        cfg.AgentName,
		IP:          cfg.IP,
		Description: cfg.AgentDescription,
		City:        cfg.City,
		Lat:         cfg.Lat,
		Lng:         cfg.Lng,
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

func rootMountPoint() string {
	if runtime.GOOS == "windows" {
		return "C:"
	}
	return "/"
}

// wmicValueList runs `wmic <args> /format:value` and returns all values of key.
func wmicValueList(args []string, key string) []string {
	argv := append(append([]string{}, args...), "/format:value")
	out, err := exec.Command("wmic", argv...).Output()
	if err != nil {
		return nil
	}
	var vals []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(kv[0]), key) {
			continue
		}
		if v := strings.TrimSpace(kv[1]); v != "" {
			vals = append(vals, v)
		}
	}
	return vals
}

func detectMemSpeed() int {
	if runtime.GOOS == "windows" {
		return detectMemSpeedWindows()
	}
	return detectMemSpeedLinux()
}

func detectMemSpeedLinux() int {
	if _, err := exec.LookPath("dmidecode"); err != nil {
		return 0
	}
	out, err := exec.Command("dmidecode", "-t", "memory").Output()
	if err != nil {
		return 0
	}
	maxSpeed := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Speed:") {
			fields := strings.Fields(strings.TrimPrefix(line, "Speed:"))
			if len(fields) == 0 {
				continue
			}
			if speed, err := strconv.Atoi(fields[0]); err == nil && speed > maxSpeed {
				maxSpeed = speed
			}
		}
	}
	return maxSpeed
}

func detectMemSpeedWindows() int {
	maxSpeed := 0
	for _, v := range wmicValueList([]string{"memorychip", "get", "Speed"}, "Speed") {
		if speed, err := strconv.Atoi(v); err == nil && speed > maxSpeed {
			maxSpeed = speed
		}
	}
	return maxSpeed
}

// baseBlockDevice strips partition suffix: sda1->sda, nvme0n1p2->nvme0n1,
// mmcblk0p1->mmcblk0, vda1->vda, xvda1->xvda.
func baseBlockDevice(dev string) string {
	name := strings.TrimPrefix(dev, "/dev/")
	if strings.HasPrefix(name, "nvme") {
		if p := strings.LastIndex(name, "p"); p > 0 {
			if _, err := strconv.Atoi(name[p+1:]); err == nil {
				return name[:p]
			}
		}
		return name
	}
	if strings.HasPrefix(name, "mmcblk") {
		if p := strings.Index(name, "p"); p > 0 {
			if _, err := strconv.Atoi(name[p+1:]); err == nil {
				return name[:p]
			}
		}
		return name
	}
	i := len(name)
	for i > 0 && name[i-1] >= '0' && name[i-1] <= '9' {
		i--
	}
	if i < len(name) && i > 0 {
		return name[:i]
	}
	return name
}

var (
	netMu       sync.Mutex
	netPrevDown uint64
	netPrevUp   uint64
	netPrevTime time.Time
	netSampled  bool
)

// netSpeed returns down/up throughput in Mbps since the previous call
// (zeros on the first call). Loopback interfaces are excluded.
func netSpeed() (down, up float64) {
	counters, err := gnet.IOCounters(true)
	if err != nil {
		return 0, 0
	}
	var totalDown, totalUp uint64
	for _, c := range counters {
		iface, err := net.InterfaceByName(c.Name)
		if err != nil || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		totalDown += c.BytesRecv
		totalUp += c.BytesSent
	}
	now := time.Now()
	netMu.Lock()
	defer netMu.Unlock()
	if netSampled && now.After(netPrevTime) {
		dt := now.Sub(netPrevTime).Seconds()
		if totalDown >= netPrevDown {
			down = float64(totalDown-netPrevDown) * 8 / 1e6 / dt
		}
		if totalUp >= netPrevUp {
			up = float64(totalUp-netPrevUp) * 8 / 1e6 / dt
		}
	}
	netPrevDown, netPrevUp, netPrevTime, netSampled = totalDown, totalUp, now, true
	return down, up
}

func sendOnce(ctx context.Context, centralURL, token string, report Report) error {
	body, _ := json.Marshal(report)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, centralURL+"/api/v1/agents/heartbeat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return &statusError{res.StatusCode}
	}
	return nil
}

type statusError struct {
	code int
}

func (e *statusError) Error() string {
	return "heartbeat rejected: " + http.StatusText(e.code)
}

func Start(ctx context.Context, logger *slog.Logger, cfg *config.Config, hostname string, db *sql.DB, dock *docker.Client) {
	ticker := time.NewTicker(cfg.Heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report, _ := Collect(ctx, db, dock, cfg, hostname)
			if err := sendOnce(ctx, cfg.CentralURL, cfg.Token, report); err != nil {
				logger.Warn("heartbeat failed", "error", err)
			}
		}
	}
}