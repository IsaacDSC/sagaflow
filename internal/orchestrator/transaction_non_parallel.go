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

type DataAggregator struct {
	Data  any   `json:"data"`
	Error error `json:"error,omitempty"`
}

func (t TransactionNonParallel) Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input, conf rule.Configs) (map[string]DataAggregator, error) {
	l := logger.FromContext(ctx)

	aggregator := make(map[string]DataAggregator)
	aggregator["sagaflow"] = DataAggregator{
		Data:  payload.Data,
		Error: nil,
	}

	for _, transactionData := range transactions {
		resp, err := t.publisher.Request(ctx, transactionData, payload, conf)

		aggregator[transactionData.ServiceName] = DataAggregator{
			Data:  resp,
			Error: err,
		}

		if err != nil {
			l.Error("error on consumer transaction", "error", err, "tag", "TransactionNonParallel.Execute")
			return aggregator, fmt.Errorf("%w: %v", ErrorConsumerTransaction, err)
		}
	}

	return aggregator, nil
}
