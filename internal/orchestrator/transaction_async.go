package orchestrator

import (
	"context"
	"errors"
	"strings"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/gqueue"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	"github.com/google/uuid"
)

// TransactionAsync publishes each saga step to gqueue (POST /api/v1/pubsub) instead of invoking HTTP endpoints directly.
type TransactionAsync struct {
	queue                gqueue.API
	publisherServiceName string
}

// NewTransactionAsync returns a use case that uses the given gqueue client. publisherServiceName is the
// prefix used in PublishInput.ServiceName as "{publisherServiceName}:rule:{ruleName}" (defaults to "sagaflow" when empty).
func NewTransactionAsync(queue gqueue.API, publisherServiceName string) *TransactionAsync {
	name := strings.TrimSpace(publisherServiceName)
	if name == "" {
		name = "sagaflow"
	}
	return &TransactionAsync{
		queue:                queue,
		publisherServiceName: name,
	}
}

// Execute publishes the saga payload to gqueue (POST /api/v1/pubsub).
func (t *TransactionAsync) Execute(ctx context.Context, ruleName string, payload Input, conf rule.Configs) error {
	if t == nil || t.queue == nil {
		return ErrorAsyncNotConfigured
	}
	l := logger.FromContext(ctx)

	rn := strings.TrimSpace(ruleName)

	data := map[string]any{"payload": payload.Data}
	err := t.queue.Publish(ctx, gqueue.PublishInput{
		EventName:   rn,
		ServiceName: t.publisherServiceName,
		Data:        data,
		Metadata: map[string]any{
			"transaction_id":  payload.TransactionID.String(),
			"orchestrator_id": payload.OrchestratorID.String(),
			"request_id":      uuid.New().String(),
		},
	})

	if err != nil {
		l.Error(
			"gqueue publish failed",
			"error", err,
			"rule_name", ruleName,
			"tag", "TransactionAsync.Execute",
		)
		return errors.Join(ErrorConsumerTransaction, err)
	}

	return nil
}
