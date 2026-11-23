package heartbeat

import (
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

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
