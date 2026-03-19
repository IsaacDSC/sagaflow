package orchestrator

import (
	"context"
	"fmt"
	"log"
	"sync"

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
		ConfigRules    []rule.HTTPConfig
	}
	RollbackParallel struct {
		psqlStore PsqlStore
		publisher Publisher
	}
)

func NewRollbackParallel(psqlStore PsqlStore, publisher Publisher) *RollbackParallel {
	return &RollbackParallel{
		psqlStore: psqlStore,
		publisher: publisher,
	}
}

func (r RollbackParallel) Execute(ctx context.Context, rollbackRules []rule.HTTPConfig, payload Input) error {
	l := logger.FromContext(ctx)
	var (
		wg   sync.WaitGroup
		errs = make(chan error, len(rollbackRules))
		done = make(chan struct{})
	)

	for _, transactionData := range rollbackRules {
		wg.Add(1)
		go func(txData rule.HTTPConfig) {
			defer wg.Done()
			err := r.publisher.Send(ctx, txData, payload, rule.Configs{
				MaxRetry:   3,
				MaxTimeout: "1m",
				Parallel:   true,
				Sync:       false,
			})
			if err != nil {
				log.Printf("error on rollback transaction with url %s: %v", txData.URL, err)
				errs <- err
			}
		}(transactionData)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errs:
		if err := r.psqlStore.SaveTransaction(ctx, Transaction{
			TransactionID:  payload.TransactionID,
			OrchestratorID: payload.OrchestratorID,
			Data:           payload.Data,
			Headers:        payload.Headers,
			Error:          err,
			ConfigRules:    rollbackRules,
		}, err.Error()); err != nil {
			l.Error("error saving transaction", "error", err, "tag", "Orchestrator.Rollback.parallel")
			return ErrorSaveTransaction
		}

		l.Error("error on rollback transaction", "error", err, "tag", "Orchestrator.Rollback.parallel")
		return fmt.Errorf("%w: %v", ErrorTransactionRollback, err)
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
