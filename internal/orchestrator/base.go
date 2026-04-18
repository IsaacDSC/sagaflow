package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/cfg"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/gqueue"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	"github.com/google/uuid"
)

var (
	ErrorConsumerTransaction = errors.New("error on consumer transaction")
	ErrorRuleNotFound        = errors.New("rule not found")
	ErrorTransactionRollback = errors.New("error when transaction rollback") // when the transaction is rollback with error
	ErrorTransactionFailed   = errors.New("error transaction failed")        // when the transaction is not successful
	ErrorSaveTransaction     = errors.New("error saving transaction")        // when the transaction with rollback error is not saved in the database
	// ErrorAsyncNotConfigured is returned when the rule uses async mode (Configs.Sync == false) but no gqueue client was wired (see TransactionAsync / pkg/gqueue).
	ErrorAsyncNotConfigured = errors.New("async transaction requires gqueue client; set GQUEUE_BASE_URL")
	// ErrorSaveRule wraps persistence failures from UpsertRule (rule store Save).
	ErrorSaveRule = errors.New("save rule")
	// ErrorGqueueUpsertRule wraps gqueue UpsertEventConsumer failures from UpsertRule.
	ErrorGqueueUpsertRule = errors.New("gqueue upsert rule")
)

type (
	MemStore interface {
		Find(ctx context.Context, txID uuid.UUID) (rule.Rule, error)
	}

	// RuleStore persists saga rules (e.g. PostgreSQL via store.Psql).
	RuleStore interface {
		Save(ctx context.Context, r rule.Rule) (uuid.UUID, error)
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
		store     Store
		gqueueAPI gqueue.API
		// stores
		memStore  MemStore
		ruleStore RuleStore
		// use cases
		transactionParallel    TransactionUseCase
		transactionNonParallel TransactionAggregatorUseCase
		transactionAsync       TransactionAsyncUseCase
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

	// TransactionAsyncUseCase publishes saga steps to gqueue when rule.Configs.Sync is false.
	TransactionAsyncUseCase interface {
		Execute(ctx context.Context, serviceName string, payload Input, conf rule.Configs) error
	}
)

func New(
	memStore MemStore,
	psqlStore Store,
	transactionParallel TransactionUseCase,
	transactionNonParallel TransactionAggregatorUseCase,
	transactionAsync TransactionAsyncUseCase,
	rollbackParallel RollbackUseCase,
	gqueueAPI gqueue.API,
	ruleStore RuleStore,
) *Orchestrator {
	return &Orchestrator{
		memStore:               memStore,
		store:                  psqlStore,
		transactionParallel:    transactionParallel,
		transactionNonParallel: transactionNonParallel,
		transactionAsync:       transactionAsync,
		rollbackParallel:       rollbackParallel,
		gqueueAPI:              gqueueAPI,
		ruleStore:              ruleStore,
	}
}

// UpsertRule registers async rules in gqueue when needed, then persists the rule.
func (o Orchestrator) UpsertRule(ctx context.Context, rl rule.Rule) (rule.Rule, error) {
	if !rl.Configs.Sync {
		if o.gqueueAPI == nil {
			return rl, ErrorAsyncNotConfigured
		}
		if err := upsertRuleInGqueue(ctx, o.gqueueAPI, rl); err != nil {
			return rl, fmt.Errorf("%w: %w", ErrorGqueueUpsertRule, err)
		}
	}
	if o.ruleStore == nil {
		return rl, fmt.Errorf("%w: rule store not configured", ErrorSaveRule)
	}
	rl.ID = uuid.New()
	ruleID, err := o.ruleStore.Save(ctx, rl)
	if err != nil {
		return rl, fmt.Errorf("%w: %w", ErrorSaveRule, err)
	}
	rl.ID = ruleID
	return rl, nil
}

func upsertRuleInGqueue(ctx context.Context, api gqueue.API, rl rule.Rule) error {
	opt := gqueue.EventOption{
		WQType:     "low_throughput",
		MaxRetries: rl.Configs.MaxRetry,
		Retention:  gqueueRetention(rl.Configs.MaxTimeout),
	}
	b := gqueue.NewEventConsumerBuilder(rl.Name).
		WithEventType("external").
		WithOption(opt)

	for _, tx := range rl.Transactions {
		host, path, err := splitConsumerURL(tx.URL)
		if err != nil {
			return fmt.Errorf("transaction url %q: %w", tx.URL, err)
		}
		cb := gqueue.NewConsumerBuilder().
			WithServiceName(tx.ServiceName).
			WithConsumerType("persistent").
			WithHost(host).
			WithPath(path)
		var err2 error
		b, err2 = b.AddConsumerFromBuilder(cb)
		if err2 != nil {
			return err2
		}
	}

	input, err := b.Build()
	if err != nil {
		return err
	}
	return api.UpsertEventConsumer(ctx, input)
}

func gqueueRetention(maxTimeout string) string {
	if strings.TrimSpace(maxTimeout) != "" {
		return maxTimeout
	}
	return "168h"
}

func splitConsumerURL(raw string) (host, path string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", fmt.Errorf("url must include scheme and host")
	}
	host = u.Scheme + "://" + u.Host
	path = u.Path
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path = path + "?" + u.RawQuery
	}
	return host, path, nil
}

func (o Orchestrator) Transaction(ctx context.Context, txInput Input) (map[string]DataAggregator, error) {
	output := make(map[string]DataAggregator)

	orchestrator, err := o.memStore.Find(ctx, txInput.OrchestratorID)
	if err != nil {
		return output, err
	}

	// Async: rule.Configs.Sync == false — saga steps are enqueued via gqueue (POST /api/v1/pubsub), implemented by TransactionAsyncUseCase (*TransactionAsync + pkg/gqueue).
	if !orchestrator.Configs.Sync {
		if o.transactionAsync == nil {
			return output, ErrorAsyncNotConfigured
		}
		err = o.transactionAsync.Execute(ctx, orchestrator.Name, txInput, orchestrator.Configs)
	} else if orchestrator.Configs.Parallel {
		err = o.transactionParallel.Execute(ctx, orchestrator.Transactions, txInput, orchestrator.Configs)
		if errors.Is(err, ErrorConsumerTransaction) {
			err = o.rollbackParallel.Execute(ctx, orchestrator.Rollback, txInput)
		}
	} else {
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
