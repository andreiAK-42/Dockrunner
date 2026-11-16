// Package victoriametrics — клиент чтения метрик из VictoriaMetrics.
// Записывают метрики сами агенты (Influx line protocol, POST /write),
// центральный сервер только читает агрегаты через MetricsQL (/api/v1/query_range).
// Приёмник может стоять за vmauth — тогда нужен Bearer-токен.
package victoriametrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	client  *http.Client
}

// New собирает клиента. rawURL — базовый адрес, например http://victoria:8428.
func New(rawURL, token string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(rawURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}
}

// Ping проверяет доступность приёмника.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("victoria metrics not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return fmt.Errorf("victoria metrics health: status %d", res.StatusCode)
	}
	return nil
}

// DailyPoint — среднесуточная загрузка CPU/RAM кластера (проценты).
type DailyPoint struct {
	Day string  `json:"day"`
	CPU float64 `json:"cpu"`
	RAM float64 `json:"ram"`
}

// queryRange выполняет MetricsQL range-запрос и возвращает значения
// усреднённые по календарным дням (день -> среднее значение серий за день).
func (c *Client) queryRangeByDay(ctx context.Context, query string, days int) (map[string]float64, error) {
	now := time.Now()
	start := now.AddDate(0, 0, -days)
	params := url.Values{}
	params.Set("query", query)
	params.Set("start", start.Format(time.RFC3339))
	params.Set("end", now.Format(time.RFC3339))
	params.Set("step", "6h")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/query_range?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("victoria metrics query: status %d", res.StatusCode)
	}

	var payload struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Values [][]any `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("victoria metrics query: status %q", payload.Status)
	}

	// day -> [sum, count] по всем сериям и точкам дня
	type acc struct {
		sum   float64
		count int
	}
	byDay := map[string]*acc{}
	for _, series := range payload.Data.Result {
		for _, pair := range series.Values {
			if len(pair) != 2 {
				continue
			}
			ts, ok := pair[0].(float64)
			if !ok {
				continue
			}
			val, err := strconv.ParseFloat(fmt.Sprint(pair[1]), 64)
			if err != nil {
				continue
			}
			day := time.Unix(int64(ts), 0).Format("02.01")
			a, exists := byDay[day]
			if !exists {
				a = &acc{}
				byDay[day] = a
			}
			a.sum += val
			a.count++
		}
	}
	out := make(map[string]float64, len(byDay))
	for day, a := range byDay {
		if a.count > 0 {
			out[day] = a.sum / float64(a.count)
		}
	}
	return out, nil
}

// QueryClusterDaily возвращает среднесуточный CPU и RAM (в процентах)
// по кластеру за последние days дней. Метрики пишут агенты:
// server_metrics_cpu / server_metrics_mem (mem — процент занятой RAM).
func (c *Client) QueryClusterDaily(ctx context.Context, days int) ([]DailyPoint, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	cpu, err := c.queryRangeByDay(ctx, `avg(server_metrics_cpu)`, days)
	if err != nil {
		return nil, err
	}
	ram, err := c.queryRangeByDay(ctx, `avg(server_metrics_mem)`, days)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	order := []string{}
	for day := range cpu {
		if !seen[day] {
			seen[day] = true
			order = append(order, day)
		}
	}
	for day := range ram {
		if !seen[day] {
			seen[day] = true
			order = append(order, day)
		}
	}
	// Хронологический порядок дат в формате DD.MM текущего года.
	sortDays(order)

	out := make([]DailyPoint, 0, len(order))
	for _, day := range order {
		out = append(out, DailyPoint{Day: day, CPU: round2(cpu[day]), RAM: round2(ram[day])})
	}
	return out, nil
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// sortDays сортирует даты в формате DD.MM текущего года хронологически.
func sortDays(days []string) {
	type keyed struct {
		day string
		t   time.Time
	}
	year := time.Now().Year()
	arr := make([]keyed, len(days))
	for i, d := range days {
		t, err := time.Parse("02.01.2006", d+"."+strconv.Itoa(year))
		if err != nil {
			t = time.Now()
		}
		arr[i] = keyed{day: d, t: t}
	}
	for i := 1; i < len(arr); i++ {
		for j := i; j > 0 && arr[j].t.Before(arr[j-1].t); j-- {
			arr[j], arr[j-1] = arr[j-1], arr[j]
		}
	}
	for i := range arr {
		days[i] = arr[i].day
	}
}
