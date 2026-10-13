package geo

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"log/slog"
)

type Location struct {
	City string
	Lat  float64
	Lng  float64
}

func Detect(ctx context.Context, logger *slog.Logger) Location {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://ip-api.com/json/?fields=status,city,lat,lon", nil)
	if err != nil {
		return Location{}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Warn("geo detect failed", "error", err)
		return Location{}
	}
	defer func() { _ = res.Body.Close() }()
	var payload struct {
		Status string  `json:"status"`
		City   string  `json:"city"`
		Lat    float64 `json:"lat"`
		Lon    float64 `json:"lon"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil || payload.Status != "success" {
		return Location{}
	}
	return Location{City: payload.City, Lat: payload.Lat, Lng: payload.Lon}
}
