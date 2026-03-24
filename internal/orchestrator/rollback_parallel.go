package orchestrator

import (
	"context"
	"sync"

	"github.com/IsaacDSC/sagaflow/internal/cfg"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	"github.com/google/uuid"
)

type (
	Transaction struct {
		TransactionID  uuid.UUID
		OrchestratorID uuid.UUID
		Data           any
		Headers        map[string][]string
		Error          error
		Retries        int
		ConfigRules    []rule.HTTPConfig
	}
	RollbackParallel struct {
		store     Store
		publisher Publisher
	}
)

func NewRollbackParallel(psqlStore Store, publisher Publisher) *RollbackParallel {
	return &RollbackParallel{
		store:     psqlStore,
		publisher: publisher,
	}
}

func (r RollbackParallel) Execute(ctx context.Context, rollbackRules []rule.HTTPConfig, payload Input) error {
	l := logger.FromContext(ctx)

	var (
		wg   sync.WaitGroup
		errs = sync.Map{}
	)

	rollbackConf := cfg.Get().Rollback

	for _, transactionData := range rollbackRules {
		wg.Add(1)
		go func(txData rule.HTTPConfig) {
			defer wg.Done()
			err := r.publisher.Send(ctx, txData, payload, rule.Configs{
				MaxRetry:   rollbackConf.MaxRetry,
				MaxTimeout: rollbackConf.MaxTimeout,
				Parallel:   true,
				Sync:       false,
			})
			if err != nil {
				l.Error("error on rollback transaction", "error", err, "tag", "Orchestrator.Rollback.parallel")
				errs.Store(txData.URL, err)
			}
		}(transactionData)
	}

	wg.Wait()

	var totalErrs int
	errs.Range(func(transactionID, error any) bool {
		totalErrs++
		return true
	})

	if totalErrs > 0 {
		return ErrorTransactionRollback
	}

	return nil
}
