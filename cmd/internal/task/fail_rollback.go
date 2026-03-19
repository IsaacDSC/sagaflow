package task

import (
	"context"
	"time"

	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type Service interface {
	Rollback(ctx context.Context) error
}

func FailRollbacks(ctx context.Context, service Service) {
	ticker := time.NewTicker(10 * time.Second)
	logger.Info(ctx, "starting rollback transactions", "interval", time.Minute)
	for {
		select {
		case <-ticker.C:
			if err := service.Rollback(ctx); err != nil {
				logger.Error(ctx, "failed to rollback transaction", "error", err)
				continue
			}

			logger.Debug(ctx, "rolling back transactions", "interval", time.Minute)

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
