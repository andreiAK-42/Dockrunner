package pricing

import (
	"context"
	"errors"
	"math"

	"go.uber.org/zap"
)

type Service struct {
	repo   *Repository
	logger *zap.Logger
}

func NewService(repo *Repository, logger *zap.Logger) *Service {
	return &Service{repo: repo, logger: logger}
}

func (s *Service) List(ctx context.Context) ([]Rates, error) {
	return s.repo.List(ctx)
}

func (s *Service) RatesFor(ctx context.Context, agentID string) (Rates, error) {
	return s.repo.GetFor(ctx, agentID)
}

func (s *Service) SetRates(ctx context.Context, rates Rates) error {
	if rates.AgentID == "" {
		return errors.New("agent_id is required")
	}
	if rates.CpuPerCoreHour < 0 || rates.MemPerGBHour < 0 || rates.DiskPerGBHour < 0 {
		return errors.New("rates must not be negative")
	}
	return s.repo.Upsert(ctx, rates)
}

// CalcPrice считает стоимость аренды: (ядра*тариф + память*тариф + диск*тариф) * часы.
func CalcPrice(rates Rates, cpuCores, memoryGB, diskGB, hours float64) float64 {
	hourly := cpuCores*rates.CpuPerCoreHour + memoryGB*rates.MemPerGBHour + diskGB*rates.DiskPerGBHour
	return math.Round(hourly*hours*100) / 100
}
