package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/google/uuid"
)

var (
	ErrorConsumerTransaction = errors.New("error on consumer transaction")
	ErrorRuleNotFound        = errors.New("rule not found")
)

type (
	Store interface {
		Find(ctx context.Context, txID uuid.UUID) (rule.Rule, error)
	}

	Queue interface {
		Send(ctx context.Context, url string, tx Input, conf rule.Configs) error
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
	Orchestrator, err := o.repository.Find(ctx, txInput.OrchestratorID)
	if err != nil {
		return err
	}

	if Orchestrator.Configs.Parallel {
		err = o.parallel(ctx, Orchestrator, txInput, false)
		if err != nil {
			if err := o.parallel(ctx, Orchestrator, txInput, true); err != nil {
				return err
			}
		}

		return nil
	}

	idxRollback, err := o.nonParallel(ctx, Orchestrator, txInput, false)
	if err != nil {
		Orchestrator.Rollback = Orchestrator.Rollback[:idxRollback]
		if _, err := o.nonParallel(ctx, Orchestrator, txInput, true); err != nil {
			return err
		}
	}

	return nil
}

func (o Orchestrator) parallel(ctx context.Context, Orchestrator rule.Rule, payload Input, isRollback bool) error {
	var (
		wg   sync.WaitGroup
		errs = make(chan error, len(Orchestrator.Transactions))
		done = make(chan struct{})
	)

	var transactions []map[string]string
	if isRollback {
		transactions = Orchestrator.Rollback
	} else {
		transactions = Orchestrator.Transactions
	}

	for _, transactionData := range transactions {
		wg.Add(1)
		go func(txData map[string]string) {
			defer wg.Done()
			err := o.gateway.Send(ctx, txData["url"], payload, Orchestrator.Configs)
			if err != nil {
				log.Printf("error sending transaction is rollback %t with url %s: %v", isRollback, txData["url"], err)
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
	var transactions []map[string]string
	if isRollback {
		transactions = Orchestrator.Rollback
	} else {
		transactions = Orchestrator.Transactions
	}

	for i, transactionData := range transactions {
		err := o.gateway.Send(ctx, transactionData["url"], tx, Orchestrator.Configs)
		if err != nil {
			return i, err
		}
	}

	return len(transactions), nil
}
