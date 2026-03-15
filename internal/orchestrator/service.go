package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	"github.com/google/uuid"
)

var (
	ErrorConsumerTransaction = errors.New("error on consumer transaction")
	ErrorRuleNotFound        = errors.New("rule not found")
	ErrorTransactionRollback = errors.New("error when transaction rollback") // when the transaction is rollback with error
	ErrorTransactionFailed   = errors.New("error transaction failed")        // when the transaction is not successful
)

type (
	Store interface {
		Find(ctx context.Context, txID uuid.UUID) (rule.Rule, error)
	}

	Queue interface {
		Send(ctx context.Context, httpCfg rule.HTTPConfig, tx Input, conf rule.Configs) error
	}
)

type (
	Input struct {
		OrchestratorID uuid.UUID
		Data           any
		Headers        map[string][]string
	}

	Orchestrator struct {
		repository Store
		gateway    Queue
	}
)

func New(repository Store, gateway Queue) *Orchestrator {
	return &Orchestrator{
		repository: repository,
		gateway:    gateway,
	}
}

func (o Orchestrator) Execute(ctx context.Context, txInput Input) error {
	l := logger.FromContext(ctx)
	Orchestrator, err := o.repository.Find(ctx, txInput.OrchestratorID)
	if err != nil {
		return err
	}

	if Orchestrator.Configs.Parallel {
		err = o.parallel(ctx, Orchestrator, txInput, false)
		if err != nil {
			if err := o.parallel(ctx, Orchestrator, txInput, true); err != nil {
				return ErrorTransactionRollback
			}
			return ErrorTransactionFailed
		}
		return nil
	}

	idxRollback, err := o.nonParallel(ctx, Orchestrator, txInput, false)
	if err != nil {
		l.Warn("error on non parallel transaction", "error", err)
		Orchestrator.Rollback = Orchestrator.Rollback[:idxRollback]
		if _, err := o.nonParallel(ctx, Orchestrator, txInput, true); err != nil {
			return ErrorTransactionRollback
		}
		return ErrorTransactionFailed
	}

	return nil
}

func (o Orchestrator) parallel(ctx context.Context, Orchestrator rule.Rule, payload Input, isRollback bool) error {
	var (
		wg   sync.WaitGroup
		errs = make(chan error, len(Orchestrator.Transactions))
		done = make(chan struct{})
	)

	var transactions []rule.HTTPConfig
	if isRollback {
		transactions = Orchestrator.Rollback
	} else {
		transactions = Orchestrator.Transactions
	}

	for _, transactionData := range transactions {
		wg.Add(1)
		go func(txData rule.HTTPConfig) {
			defer wg.Done()
			err := o.gateway.Send(ctx, txData, payload, Orchestrator.Configs)
			if err != nil {
				log.Printf("error sending transaction is rollback %t with url %s: %v", isRollback, txData.URL, err)
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
		return fmt.Errorf("%w: %v", ErrorConsumerTransaction, err)
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (o Orchestrator) nonParallel(ctx context.Context, Orchestrator rule.Rule, tx Input, isRollback bool) (int, error) {
	var transactions []rule.HTTPConfig
	if isRollback {
		transactions = Orchestrator.Rollback
	} else {
		transactions = Orchestrator.Transactions
	}

	for i, transactionData := range transactions {
		err := o.gateway.Send(ctx, transactionData, tx, Orchestrator.Configs)
		if err != nil {
			return i, err
		}
	}

	return len(transactions), nil
}
