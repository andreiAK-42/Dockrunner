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
	AgentID       string        `json:"agent_id"`
	Hostname      string        `json:"hostname"`
	Name          string        `json:"name"`
	IP            string        `json:"ip"`
	Description   string        `json:"description"`
	City          string        `json:"city"`
	Lat           float64       `json:"lat"`
	Lng           float64       `json:"lng"`
	ApiURL        string        `json:"api_url"`
	CPUPercent    float64       `json:"cpu_percent"`
	CPUTotal      int           `json:"cpu_total"`
	CPUMhz        int           `json:"cpu_mhz"`
	MemUsedGB     float64       `json:"mem_used_gb"`
	MemTotalGB    float64       `json:"mem_total_gb"`
	MemType       string        `json:"mem_type"`
	MemSpeedMhz   int           `json:"mem_speed_mhz"`
	DiskUsedGB    float64       `json:"disk_used_gb"`
	DiskTotalGB   float64       `json:"disk_total_gb"`
	DiskType      string        `json:"disk_type"`
	Containers    int           `json:"containers"`
	Storage       []UserStorage `json:"storage"`
	NetDownMbps   float64       `json:"net_down_mbps"`
	NetUpMbps     float64       `json:"net_up_mbps"`
}

func toGB(bytes float64) float64 {
	return bytes / 1024 / 1024 / 1024
}

// resolveIP возвращает IPv4 сервера (адрес, по которому пользователи
// достучатся до опубликованных портов).
//  1. Явный AGENT_IP из .env — единственный надёжный вариант, если агент
//     в docker (из контейнера виден только его 172.17.0.x).
//  2. Публичный IP исходящего маршрута.
//  3. Лучший интерфейс: виртуальные (docker/veth/tailscale/...) пропускаются,
//     среди остальных предпочитается публичный адрес.
func resolveIP(publicIP string) string {
	if publicIP != "" {
		return publicIP
	}
	if ip := dialOutIP(); ip != "" {
		if parsed := net.ParseIP(ip); parsed != nil && isPublicIPv4(parsed) {
			return ip
		}
	}
	return bestInterfaceIP()
}

// dialOutIP узнаёт исходящий IP через UDP-dial (пакеты не отправляются).
func dialOutIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.IsLoopback() {
		return ""
	}
	return addr.IP.String()
}

// isVirtualIface — docker-мосты, veth-пары, VPN-туннели и т.п.
// Их адреса (172.17.0.x, 10.x tailscale) пользователям не нужны.
func isVirtualIface(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{
		"docker", "veth", "br-", "virbr", "vnet",
		"tailscale", "tun", "tap", "wg", "cni",
		"flannel", "cali", "kube",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isPublicIPv4(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	if v4.IsLoopback() || v4.IsLinkLocalUnicast() || v4.IsMulticast() {
		return false
	}
	if v4.IsPrivate() {
		return false
	}
	if v4[0] == 100 && v4[1] >= 64 && v4[1] < 128 {
		return false // CGNAT (tailscale)
	}
	if v4[0] == 0 || v4[0] >= 224 {
		return false
	}
	return true
}

// bestInterfaceIP — первый публичный IPv4 среди физических интерфейсов,
// иначе первый приватный (для чисто локальных сетей).
func bestInterfaceIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	fallback := ""
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if isVirtualIface(iface.Name) {
			continue
		}
		ip := ifaceIPv4(iface)
		if ip == "" {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.IsLinkLocalUnicast() {
			continue
		}
		if isPublicIPv4(parsed) {
			return ip
		}
		if fallback == "" {
			fallback = ip
		}
	}
	return fallback
}

func ifaceIPv4(iface net.Interface) string {
	addrs, err := iface.Addrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	return ""
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
	if items, err := os.ReadDir(workspace); err == nil {
		for _, item := range items {
			if !item.IsDir() {
				continue
			}
			owner := item.Name()
			entry := storageEntry(byOwner, owner)
			if size, err := fsx.DirSize(workspace + "/" + owner); err == nil {
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

func Collect(ctx context.Context, db *sql.DB, dock *docker.Client, workspace, agentID, hostname, agentName, description, city string, lat, lng float64, apiURL, publicIP string) (Report, error) {
	report := Report{
		AgentID: agentID, Hostname: hostname, Name: agentName, IP: resolveIP(publicIP),
		Description: description, City: city, Lat: lat, Lng: lng, ApiURL: apiURL,
	}
	if percents, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false); err == nil && len(percents) > 0 {
		report.CPUPercent = percents[0]
	}
	if total, err := cpu.CountsWithContext(ctx, true); err == nil {
		report.CPUTotal = total
	}
	if cpuInfo, err := cpu.InfoWithContext(ctx); err == nil && len(cpuInfo) > 0 {
		report.CPUMhz = int(cpuInfo[0].Mhz)
	}
	if m, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		report.MemUsedGB = toGB(float64(m.Used))
		report.MemTotalGB = toGB(float64(m.Total))
		report.MemType = detectMemType()
		report.MemSpeedMhz = detectMemSpeed()
	}
	root := rootMountPoint()
	if d, err := disk.UsageWithContext(ctx, root); err == nil {
		report.DiskUsedGB = toGB(float64(d.Used))
		report.DiskTotalGB = toGB(float64(d.Total))
		report.DiskType = detectDiskType(root)
	} else if runtime.GOOS == "windows" {
		// Fallback: first available partition.
		if parts, err := disk.Partitions(false); err == nil && len(parts) > 0 {
			for _, p := range parts {
				if dd, err := disk.UsageWithContext(ctx, p.Mountpoint); err == nil {
					report.DiskUsedGB = toGB(float64(dd.Used))
					report.DiskTotalGB = toGB(float64(dd.Total))
					report.DiskType = detectDiskType(p.Mountpoint)
					break
				}
			}
		}
	}
	if ids, err := dock.RunningIDs(ctx); err == nil {
		report.Containers = len(ids)
	}
	report.Storage = collectStorage(ctx, db, dock, workspace)
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

func detectMemType() string {
	if runtime.GOOS == "windows" {
		return detectMemTypeWindows()
	}
	return detectMemTypeLinux()
}

func detectMemTypeLinux() string {
	if _, err := exec.LookPath("dmidecode"); err != nil {
		return ""
	}
	out, err := exec.Command("dmidecode", "-t", "memory").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Type:") {
			typ := strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			switch typ {
			case "DDR", "DDR2", "DDR3", "DDR4", "DDR5", "LPDDR3", "LPDDR4", "LPDDR5":
				return typ
			}
		}
	}
	return ""
}

func detectMemTypeWindows() string {
	// SMBIOSMemoryType is more precise: 26=DDR4, 34=DDR5.
	for _, v := range wmicValueList([]string{"memorychip", "get", "SMBIOSMemoryType"}, "SMBIOSMemoryType") {
		switch v {
		case "26":
			return "DDR4"
		case "34":
			return "DDR5"
		case "24":
			return "DDR3"
		case "22":
			return "DDR2"
		case "18":
			return "DDR"
		}
	}
	// MemoryType: 20=DDR, 21=DDR2, 24=DDR3, 26=DDR4, 27=DDR5
	for _, v := range wmicValueList([]string{"memorychip", "get", "MemoryType"}, "MemoryType") {
		switch v {
		case "20":
			return "DDR"
		case "21":
			return "DDR2"
		case "22":
			return "DDR2 FB-DIMM"
		case "24":
			return "DDR3"
		case "26":
			return "DDR4"
		case "27":
			return "DDR5"
		}
	}
	return ""
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

func detectDiskType(mountPoint string) string {
	if runtime.GOOS == "windows" {
		return detectDiskTypeWindows(mountPoint)
	}
	return detectDiskTypeLinux(mountPoint)
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

func detectDiskTypeLinux(mountPoint string) string {
	// 0 = SSD, 1 = HDD
	devices, err := disk.Partitions(false)
	if err != nil {
		return ""
	}
	for _, part := range devices {
		if part.Mountpoint != mountPoint {
			continue
		}
		base := baseBlockDevice(part.Device)
		if base == "" {
			continue
		}
		// NVMe / eMMC — всегда flash, даже если rotational врёт в виртуалке.
		if strings.HasPrefix(base, "nvme") || strings.HasPrefix(base, "mmcblk") {
			return "SSD"
		}
		if data, err := os.ReadFile("/sys/block/" + base + "/queue/rotational"); err == nil {
			trimmed := strings.TrimSpace(string(data))
			if trimmed == "0" {
				return "SSD"
			}
			if trimmed == "1" {
				return "HDD"
			}
		}
	}
	return ""
}

func psOutput(script string) string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectDiskTypeWindows(mountPoint string) string {
	letter := "C"
	if len(mountPoint) >= 1 {
		letter = strings.ToUpper(string(mountPoint[0]))
	}
	diskOf := `Get-Partition -DriveLetter ` + letter + ` -ErrorAction SilentlyContinue | Get-Disk -ErrorAction SilentlyContinue`

	// 1. BusType: NVMe всегда SSD.
	if bus := strings.ToUpper(psOutput(`(` + diskOf + ` | Select-Object -ExpandProperty BusType -ErrorAction SilentlyContinue)`)); bus != "" {
		if strings.Contains(bus, "NVME") {
			return "SSD"
		}
		if strings.Contains(bus, "SSD") {
			return "SSD"
		}
		if strings.Contains(bus, "HDD") {
			return "HDD"
		}
	}
	// 2. MediaType диска системного раздела.
	if mt := strings.ToUpper(psOutput(`(` + diskOf + ` | Select-Object -ExpandProperty MediaType -ErrorAction SilentlyContinue)`)); mt != "" {
		if strings.Contains(mt, "SSD") {
			return "SSD"
		}
		if strings.Contains(mt, "HDD") {
			return "HDD"
		}
		// "Unspecified"/"Fixed hard disk media" — неоднозначно, дальше.
	}
	// 3. SpindleSpeed: 0/пусто у SSD, >0 у HDD.
	if sp := strings.TrimSpace(psOutput(`(` + diskOf + ` | Select-Object -ExpandProperty SpindleSpeed -ErrorAction SilentlyContinue)`)); sp != "" {
		if sp == "0" {
			return "SSD"
		}
		if n, err := strconv.Atoi(sp); err == nil && n > 0 {
			return "HDD"
		}
	}
	// 4. WMI fallback. "Fixed hard disk media" НЕ маппим в HDD —
	// так wmic показывает и NVMe SSD. Только явные SSD/HDD.
	for _, v := range wmicValueList([]string{"diskdrive", "get", "MediaType"}, "MediaType") {
		upper := strings.ToUpper(v)
		if strings.Contains(upper, "SSD") || upper == "12" || upper == "13" {
			return "SSD"
		}
		if strings.Contains(upper, "HDD") || upper == "4" {
			return "HDD"
		}
	}
	return ""
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

func Start(ctx context.Context, logger *slog.Logger, centralURL, token, agentID, hostname, agentName, description, city string, lat, lng float64, apiURL, publicIP string, interval time.Duration, db *sql.DB, dock *docker.Client, workspace string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			report, _ := Collect(ctx, db, dock, workspace, agentID, hostname, agentName, description, city, lat, lng, apiURL, publicIP)
			if err := sendOnce(ctx, centralURL, token, report); err != nil {
				logger.Warn("heartbeat failed", "error", err)
			}
		}
	}
}
