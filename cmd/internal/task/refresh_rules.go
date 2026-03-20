package task

import (
	"context"
	"fmt"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/cfg"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

func RefreshRules(ctx context.Context, psqlStore store.PsqlImpl, memStore store.MemoryImpl) {
	conf := cfg.Get()
	ticker := time.NewTicker(conf.Task.IntervalRefreshRules)
	logger.Info(ctx, "starting rules refresh", "interval", time.Minute)
	for {
		select {
		case <-ticker.C:
			logger.Debug(ctx, "refreshing rules", "interval", time.Minute)
			if err := LoadMemRules(ctx, psqlStore, memStore); err != nil {
				logger.Error(ctx, "failed to load memory rules", "error", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func LoadMemRules(ctx context.Context, psqlStore store.PsqlImpl, memStore store.MemoryImpl) error {
	rules, err := psqlStore.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to find all rules: %w", err)
	}

	if err := memStore.Refresh(ctx, rules); err != nil {
		return fmt.Errorf("failed to refresh memory store: %w", err)
	}

	return nil
}
