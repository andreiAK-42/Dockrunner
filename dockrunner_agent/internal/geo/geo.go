package geo

import (
	"context"
	"encoding/json"
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
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "https://ipapi.co/json/", nil)
	if err != nil {
		return Location{IP: LocalIP()}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("geo detect failed", "error", err)
		return Location{IP: LocalIP()}
	}
	defer func() { _ = res.Body.Close() }()

	var payload struct {
		IP   string  `json:"ip"`
		City string  `json:"city"`
		Lat  float64 `json:"latitude"`
		Lon  float64 `json:"longitude"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		logger.Warn("geo decode failed", "error", err)
		return Location{IP: LocalIP()}
	}
	ip := payload.IP
	if ip == "" {
		ip = LocalIP()
	}
	return Location{City: payload.City, IP: ip, Lat: payload.Lat, Lng: payload.Lon}
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
