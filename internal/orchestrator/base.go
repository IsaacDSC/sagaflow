package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/cfg"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	"github.com/google/uuid"
)

var (
	ErrorConsumerTransaction = errors.New("error on consumer transaction")
	ErrorRuleNotFound        = errors.New("rule not found")
	ErrorTransactionRollback = errors.New("error when transaction rollback") // when the transaction is rollback with error
	ErrorTransactionFailed   = errors.New("error transaction failed")        // when the transaction is not successful
	ErrorSaveTransaction     = errors.New("error saving transaction")        // when the transaction with rollback error is not saved in the database
)

type (
	MemStore interface {
		Find(ctx context.Context, txID uuid.UUID) (rule.Rule, error)
	}

	Store interface {
		SaveTransaction(ctx context.Context, txData Transaction, errorMessage string) error
		GetTransactions(ctx context.Context, status rule.Status, maxRetry int) ([]Transaction, error)
		UpdateTxStatus(ctx context.Context, txID uuid.UUID, status rule.Status) error
		UpdateRetries(ctx context.Context, txID uuid.UUID, retries int) error
	}

	Publisher interface {
		Send(ctx context.Context, httpCfg rule.HTTPConfig, tx Input, conf rule.Configs) error
		Request(ctx context.Context, httpCfg rule.HTTPConfig, tx Input, conf rule.Configs) (map[string]any, error)
	}
)

type (
	Input struct {
		TransactionID  uuid.UUID
		OrchestratorID uuid.UUID
		Data           any
		Headers        map[string][]string
	}

	Orchestrator struct {
		memStore               MemStore
		store                  Store
		transactionParallel    TransactionUseCase
		transactionNonParallel TransactionAggregatorUseCase
		rollbackParallel       RollbackUseCase
	}

	TransactionUseCase interface {
		Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input, conf rule.Configs) error
	}

	TransactionAggregatorUseCase interface {
		Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input, conf rule.Configs) (map[string]DataAggregator, error)
	}

	RollbackUseCase interface {
		Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input) error
	}
)

func New(
	memStore MemStore,
	psqlStore Store,
	transactionParallel TransactionUseCase,
	transactionNonParallel TransactionAggregatorUseCase,
	rollbackParallel RollbackUseCase,
) *Orchestrator {
	return &Orchestrator{
		memStore:               memStore,
		store:                  psqlStore,
		transactionParallel:    transactionParallel,
		transactionNonParallel: transactionNonParallel,
		rollbackParallel:       rollbackParallel,
	}
}

func (o Orchestrator) Transaction(ctx context.Context, txInput Input) (map[string]DataAggregator, error) {
	output := make(map[string]DataAggregator)

	orchestrator, err := o.memStore.Find(ctx, txInput.OrchestratorID)
	if err != nil {
		return output, err
	}

	if orchestrator.Configs.Parallel {
		err = o.transactionParallel.Execute(ctx, orchestrator.Transactions, txInput, orchestrator.Configs)
		if errors.Is(err, ErrorConsumerTransaction) {
			err = o.rollbackParallel.Execute(ctx, orchestrator.Rollback, txInput)
		}
	}

	if !orchestrator.Configs.Parallel {
		output, err = o.transactionNonParallel.Execute(ctx, orchestrator.Transactions, txInput, orchestrator.Configs)
		if errors.Is(err, ErrorConsumerTransaction) {
			err = o.rollbackParallel.Execute(ctx, orchestrator.Rollback, txInput)
		}
	}

	if err != nil {
		if err := o.store.SaveTransaction(ctx, Transaction{
			TransactionID:  txInput.TransactionID,
			OrchestratorID: txInput.OrchestratorID,
			Data:           txInput.Data,
			Headers:        txInput.Headers,
			Error:          err,
			ConfigRules:    orchestrator.Rollback,
		}, err.Error()); err != nil {
			return output, ErrorSaveTransaction
		}

		return output, ErrorTransactionRollback

	}

	return output, nil
}

func (o Orchestrator) Rollback(ctx context.Context) error {
	logger.Info(ctx, "starting rollback transactions", "interval", time.Minute)
	rollbackConf := cfg.Get().Rollback

	transactions, err := o.store.GetTransactions(ctx, rule.StatusFailedExecuteRollback, rollbackConf.MaxRetry)
	if errors.Is(err, ErrorRuleNotFound) {
		return nil
	}

	if err != nil {
		logger.Error(ctx, "failed to get transactions", "error", err)
		return fmt.Errorf("failed to get transactions: %w", err)
	}

	l := logger.FromContext(ctx)
	var (
		wg   sync.WaitGroup
		errs = sync.Map{}
	)

	for _, transaction := range transactions {
		wg.Add(1)
		go func(transaction Transaction) {
			defer wg.Done()
			err := o.rollbackParallel.Execute(ctx, transaction.ConfigRules, Input{
				TransactionID:  transaction.TransactionID,
				OrchestratorID: transaction.OrchestratorID,
				Data:           transaction.Data,
				Headers:        transaction.Headers,
			})

			if err != nil {
				if err := o.store.UpdateRetries(ctx, transaction.TransactionID, transaction.Retries+1); err != nil {
					l.Error("failed to storage update retries", "error", err)
					errs.Store(transaction.TransactionID, err)
				}

				return
			}

			if err := o.store.UpdateTxStatus(ctx, transaction.TransactionID, rule.StatusRollbackExecuted); err != nil {
				l.Error("failed to storage update transaction successful status", "error", err)
				errs.Store(transaction.TransactionID, err)
			}

		}(transaction)

	}

	wg.Wait()

	var totalErrs int
	errs.Range(func(transactionID, error any) bool {
		totalErrs++
		return true
	})

	if totalErrs > 0 {
		return fmt.Errorf("failed to rollback transactions with total errors: %d", totalErrs)
	}

	return nil
}
