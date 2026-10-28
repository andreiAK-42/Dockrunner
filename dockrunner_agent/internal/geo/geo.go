package geo

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"log/slog"
)

type Location struct {
	City string
	IP   string
	Lat  float64
	Lng  float64
}

func Detect(ctx context.Context, logger *slog.Logger) Location {
	start := time.Now()
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://ipapi.co/json/", nil)
	if err != nil {
		logger.Warn("geo detect failed: request creation error", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return Location{IP: LocalIP()}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("geo detect failed: HTTP error", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return Location{IP: LocalIP()}
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		logger.Warn("geo detect failed: non-2xx status", "status", res.StatusCode, "duration_ms", time.Since(start).Milliseconds())
		return Location{IP: LocalIP()}
	}

	var payload struct {
		IP   string  `json:"ip"`
		City string  `json:"city"`
		Lat  float64 `json:"latitude"`
		Lon  float64 `json:"longitude"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&payload); err != nil {
		logger.Warn("geo decode failed", "error", err, "duration_ms", time.Since(start).Milliseconds())
		return Location{IP: LocalIP()}
	}
	if payload.IP == "" {
		logger.Warn("geo response did not contain a public IP", "duration_ms", time.Since(start).Milliseconds())
		return Location{IP: LocalIP()}
	}

	logger.Info("geo detect succeeded",
		"ip", payload.IP,
		"city", payload.City,
		"lat", payload.Lat,
		"lng", payload.Lon,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return Location{City: payload.City, IP: payload.IP, Lat: payload.Lat, Lng: payload.Lon}
}

// LocalIP — исходящий адрес машины (LAN). Нужен, если geo-сервис недоступен.
func LocalIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 2*time.Second)
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr == nil {
		return ""
	}
	return addr.IP.String()
}
