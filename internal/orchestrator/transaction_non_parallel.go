package orchestrator

import (
	"context"
	"fmt"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type (
	TransactionNonParallel struct {
		memStore  MemStore
		publisher Publisher
	}
)

func NewTransactionNonParallel(memStore MemStore, publisher Publisher) *TransactionNonParallel {
	return &TransactionNonParallel{
		memStore:  memStore,
		publisher: publisher,
	}
}

func (t TransactionNonParallel) Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input, conf rule.Configs) error {
	l := logger.FromContext(ctx)

	for _, transactionData := range transactions {
		err := t.publisher.Send(ctx, transactionData, payload, conf)
		if err != nil {
			l.Error("error on consumer transaction", "error", err, "tag", "TransactionNonParallel.Execute")
			return fmt.Errorf("%w: %v", ErrorConsumerTransaction, err)
		}
	}

	return nil
}
