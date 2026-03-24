package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type (
	TransactionParallel struct {
		memStore  MemStore
		publisher Publisher
	}
)

func NewTransactionParallel(memStore MemStore, publisher Publisher) *TransactionParallel {
	return &TransactionParallel{
		memStore:  memStore,
		publisher: publisher,
	}
}

func (t TransactionParallel) Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input, conf rule.Configs) error {
	l := logger.FromContext(ctx)
	var (
		wg   sync.WaitGroup
		errs = make(chan error, len(transactions))
		done = make(chan struct{})
	)

	for _, transactionData := range transactions {
		wg.Add(1)
		go func(txData rule.HTTPConfig) {
			defer wg.Done()
			err := t.publisher.Send(ctx, txData, payload, conf)
			if err != nil {
				l.Error("error sending transaction is rollback", "error", err, "tag", "Orchestrator.Transaction.parallel")
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
		if len(errs) == len(transactions) {
			return nil
		}
		return fmt.Errorf("%w: %v", ErrorConsumerTransaction, err)
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
