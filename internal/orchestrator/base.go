package orchestrator

import (
	"context"
	"database/sql"
	"errors"
	"time"

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

	PsqlStore interface {
		SaveTransaction(ctx context.Context, txData Transaction, errorMessage string) error
		GetTransactions(ctx context.Context, status string) ([]Transaction, error)
		UpdateTransaction(ctx context.Context, txID uuid.UUID) error
	}

	Publisher interface {
		Send(ctx context.Context, httpCfg rule.HTTPConfig, tx Input, conf rule.Configs) error
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
		psqlStore              PsqlStore
		transactionParallel    TransactionUseCase
		transactionNonParallel TransactionUseCase
		rollbackParallel       RollbackUseCase
	}

	TransactionUseCase interface {
		Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input, conf rule.Configs) error
	}

	RollbackUseCase interface {
		Execute(ctx context.Context, transactions []rule.HTTPConfig, payload Input) error
	}
)

func New(memStore MemStore, psqlStore PsqlStore, transactionParallel TransactionUseCase, transactionNonParallel TransactionUseCase, rollbackParallel RollbackUseCase) *Orchestrator {
	return &Orchestrator{
		memStore:               memStore,
		psqlStore:              psqlStore,
		transactionParallel:    transactionParallel,
		transactionNonParallel: transactionNonParallel,
		rollbackParallel:       rollbackParallel,
	}
}

func (o Orchestrator) Transaction(ctx context.Context, txInput Input) error {
	orchestrator, err := o.memStore.Find(ctx, txInput.OrchestratorID)
	if err != nil {
		return err
	}

	if orchestrator.Configs.Parallel {
		err = o.transactionParallel.Execute(ctx, orchestrator.Transactions, txInput, orchestrator.Configs)
		if errors.Is(err, ErrorConsumerTransaction) {
			err = o.rollbackParallel.Execute(ctx, orchestrator.Rollback, txInput)
			if err != nil {
				return err
			}
		}
	}

	err = o.transactionNonParallel.Execute(ctx, orchestrator.Transactions, txInput, orchestrator.Configs)
	if errors.Is(err, ErrorConsumerTransaction) {
		err = o.rollbackParallel.Execute(ctx, orchestrator.Rollback, txInput)
		if err != nil {
			return err
		}
	}

	return nil
}

func (o Orchestrator) Rollback(ctx context.Context, transaction Transaction) error {
	logger.Info(ctx, "starting rollback transactions", "interval", time.Minute)

	transactions, err := o.psqlStore.GetTransactions(ctx, "error_execute_rollback")
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	if err != nil {
		logger.Error(ctx, "failed to get transactions", "error", err)
		return err
	}

	for _, transaction := range transactions {
		if err := o.rollbackParallel.Execute(ctx, transaction.ConfigRules, Input{
			TransactionID:  transaction.TransactionID,
			OrchestratorID: transaction.OrchestratorID,
			Data:           transaction.Data,
			Headers:        transaction.Headers,
		}); err != nil {
			return err
		}

		if err := o.psqlStore.UpdateTransaction(ctx, transaction.TransactionID); err != nil {
			return err
		}
	}

	logger.Debug(ctx, "rolling back transactions", "interval", time.Minute)

	return nil
}
