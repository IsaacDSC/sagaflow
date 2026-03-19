package task

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type Service interface {
	Rollback(ctx context.Context, transaction orchestrator.Transaction) error
}

func FailRollbacks(ctx context.Context, psqlStore store.PsqlImpl, service Service) {
	ticker := time.NewTicker(1 * time.Minute)
	logger.Info(ctx, "starting rollback transactions", "interval", time.Minute)
	for {
		select {
		case <-ticker.C:
			transactions, err := psqlStore.GetTransactions(ctx, store.StatusFailedExecuteRollback)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			if err != nil {
				logger.Error(ctx, "failed to get transactions", "error", err)
				continue
			}

			for _, transaction := range transactions {
				if err := service.Rollback(ctx, transaction); err != nil {
					logger.Error(ctx, "failed to rollback transaction", "error", err)
					continue
				}
			}

			logger.Debug(ctx, "rolling back transactions", "interval", time.Minute)

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
