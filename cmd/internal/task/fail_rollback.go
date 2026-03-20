package task

import (
	"context"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/cfg"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type Service interface {
	Rollback(ctx context.Context) error
}

func FailRollbacks(ctx context.Context, service Service) {
	conf := cfg.Get()
	ticker := time.NewTicker(conf.Task.IntervalRollback)
	logger.Info(ctx, "starting rollback transactions", "interval", time.Minute)
	for {
		select {
		case <-ticker.C:
			logger.Debug(ctx, "rolling back transactions", "interval", time.Minute)

			if err := service.Rollback(ctx); err != nil {
				logger.Error(ctx, "failed internal task to rollback transaction", "error", err)
				continue
			}

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
