package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Основной источник: ipapi.co
	var ipapi struct {
		IP   string  `json:"ip"`
		City string  `json:"city"`
		Lat  float64 `json:"latitude"`
		Lon  float64 `json:"longitude"`
	}

	err := requestJSON(ctx, client, "https://ipapi.co/json/", &ipapi)
	if err == nil && ipapi.IP != "" {
		logger.Info("Геолокация получена через ipapi.co",
			"ip", ipapi.IP,
			"city", ipapi.City,
		)

		return Location{
			IP:   ipapi.IP,
			City: ipapi.City,
			Lat:  ipapi.Lat,
			Lng:  ipapi.Lon,
		}
	}

	logger.Warn("ipapi.co недоступен", "error", err)

	// Первый резерв: ipwho.is
	var ipwho struct {
		Success bool    `json:"success"`
		IP      string  `json:"ip"`
		City    string  `json:"city"`
		Lat     float64 `json:"latitude"`
		Lon     float64 `json:"longitude"`
	}

	err = requestJSON(ctx, client, "https://ipwho.is/", &ipwho)
	if err == nil && ipwho.Success && ipwho.IP != "" {
		logger.Info("Геолокация получена через ipwho.is",
			"ip", ipwho.IP,
			"city", ipwho.City,
		)

		return Location{
			IP:   ipwho.IP,
			City: ipwho.City,
			Lat:  ipwho.Lat,
			Lng:  ipwho.Lon,
		}
	}

	logger.Warn("ipwho.is недоступен", "error", err)

	// Второй резерв: freeipapi.com
	var freeipapi struct {
		IP   string  `json:"ipAddress"`
		City string  `json:"cityName"`
		Lat  float64 `json:"latitude"`
		Lon  float64 `json:"longitude"`
	}

	err = requestJSON(
		ctx,
		client,
		"https://free.freeipapi.com/api/v1/json",
		&freeipapi,
	)

	if err == nil && freeipapi.IP != "" {
		logger.Info("Геолокация получена через freeipapi.com",
			"ip", freeipapi.IP,
			"city", freeipapi.City,
		)

		return Location{
			IP:   freeipapi.IP,
			City: freeipapi.City,
			Lat:  freeipapi.Lat,
			Lng:  freeipapi.Lon,
		}
	}

	logger.Warn("freeipapi.com недоступен", "error", err)

	// Все источники недоступны.
	logger.Warn("Не удалось определить геолокацию, используем локальный IP")

	return Location{}
}

// requestJSON выполняет HTTP-запрос и декодирует JSON-ответ.
func requestJSON(
	ctx context.Context,
	client *http.Client,
	endpoint string,
	result any,
) error {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("unexpected HTTP status: %d", res.StatusCode)
	}

	if err := json.NewDecoder(
		io.LimitReader(res.Body, 1<<20),
	).Decode(result); err != nil {
		return fmt.Errorf("JSON decode failed: %w", err)
	}

	return nil
}