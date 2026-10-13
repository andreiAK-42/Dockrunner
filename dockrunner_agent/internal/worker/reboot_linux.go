//go:build linux

package worker

import (
	"syscall"
	"time"

	"log/slog"
)

func canReboot() bool {
	return true
}

func delayedReboot(logger *slog.Logger) {
	time.Sleep(5 * time.Second)
	logger.Info("rebooting host")
	_ = syscall.Reboot(syscall.LINUX_REBOOT_CMD_RESTART)
}
