//go:build !linux

package worker

import (
	"errors"

	"log/slog"
)

func canReboot() bool {
	return false
}

func delayedReboot(logger *slog.Logger) {
	logger.Error("host reboot not supported", "error", errors.New("linux only"))
}
