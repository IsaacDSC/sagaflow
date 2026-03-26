package orchestrator_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/mocks/mockstore"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

/*
	Test cases for parallel orchestrator:
	Parallel:
		- Success
		- Error on transaction with rollback
		- Error on transaction and rollback

*/

var (
	parallelRule = rule.Rule{
		ID:   uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"),
		Name: "parallel-saga",
		Transactions: []rule.HTTPConfig{
			{Method: "POST", URL: "http://svc1/do"},
			{Method: "POST", URL: "http://svc2/do"},
		},
		Rollback: []rule.HTTPConfig{
			{Method: "POST", URL: "http://svc1/undo"},
			{Method: "POST", URL: "http://svc2/undo"},
		},
		Configs: rule.Configs{Parallel: true},
	}
	nonParallelRule = rule.Rule{
		ID:   uuid.MustParse("550e8400-e29b-41d4-a716-446655440002"),
		Name: "sequential-saga",
		Transactions: []rule.HTTPConfig{
			{Method: "POST", URL: "http://svc1/do"},
			{Method: "POST", URL: "http://svc2/do"},
		},
		Rollback: []rule.HTTPConfig{
			{Method: "POST", URL: "http://svc1/undo"},
			{Method: "POST", URL: "http://svc2/undo"},
		},
		Configs: rule.Configs{Parallel: false},
	}
)

type fakeTxUseCase struct {
	exec func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error
}

func (f fakeTxUseCase) Execute(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error {
	if f.exec == nil {
		return nil
	}
	return f.exec(ctx, transactions, payload, conf)
}

type fakeRollbackUseCase struct {
	exec func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input) error
}

func (f fakeRollbackUseCase) Execute(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input) error {
	if f.exec == nil {
		return nil
	}
	return f.exec(ctx, transactions, payload)
}

func TestParallel(t *testing.T) {
	tests := []struct {
		name             string
		txParallelErr    error
		txNonParallelErr error
		rollbackErr      error
		expectRollback   bool
		wantErr          error
	}{
		{
			name:             "Success",
			txParallelErr:    nil,
			txNonParallelErr: nil,
			rollbackErr:      nil,
			expectRollback:   false,
			wantErr:          nil,
		},
		{
			name:             "Error on transaction with rollback",
			txParallelErr:    fmt.Errorf("%w: tx failed", orchestrator.ErrorConsumerTransaction),
			txNonParallelErr: nil,
			rollbackErr:      nil,
			expectRollback:   true,
			wantErr:          nil,
		},
		{
			name:             "Error on transaction and rollback",
			txParallelErr:    fmt.Errorf("%w: tx failed", orchestrator.ErrorConsumerTransaction),
			txNonParallelErr: fmt.Errorf("%w: tx failed later", orchestrator.ErrorConsumerTransaction), // should not be reached
			rollbackErr:      orchestrator.ErrorTransactionRollback,
			expectRollback:   true,
			wantErr:          orchestrator.ErrorTransactionRollback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockstore.NewMockPsqlImpl(ctrl)

			// Transaction() persiste erro somente quando o erro final após rollback não é nil.
			if tc.rollbackErr != nil {
				store.EXPECT().
					SaveTransaction(gomock.Any(), gomock.Any(), tc.rollbackErr.Error()).
					Return(nil).
					Times(1)
			}

			input := orchestrator.Input{
				OrchestratorID: parallelRule.ID,
				TransactionID:  uuid.New(),
				Data:           map[string]string{"key": "value"},
				Headers:        map[string][]string{"X-Request-Id": {"req-123"}},
			}

			memStore := mockstore.NewMockMemoryImpl(ctrl)
			memStore.EXPECT().
				Find(gomock.Any(), parallelRule.ID).
				Return(parallelRule, nil).
				Times(1)

			txParallel := fakeTxUseCase{
				exec: func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error {
					if payload.OrchestratorID != parallelRule.ID {
						t.Fatalf("payload.OrchestratorID = %v, want %v", payload.OrchestratorID, parallelRule.ID)
					}
					if len(transactions) != len(parallelRule.Transactions) {
						t.Fatalf("transactions len = %d, want %d", len(transactions), len(parallelRule.Transactions))
					}
					if conf.Parallel != true {
						t.Fatalf("conf.Parallel = %v, want true", conf.Parallel)
					}
					return tc.txParallelErr
				},
			}

			txNonParallel := fakeTxUseCase{
				exec: func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error {
					// In the rollback failure case, orchestrator returns early.
					if tc.rollbackErr != nil {
						t.Fatalf("transactionNonParallel.Execute called, but rollback is expected to fail first")
					}
					return tc.txNonParallelErr
				},
			}

			rollback := fakeRollbackUseCase{
				exec: func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input) error {
					if !tc.expectRollback {
						t.Fatalf("rollback.Execute called unexpectedly")
					}
					return tc.rollbackErr
				},
			}

			o := orchestrator.New(memStore, store, txParallel, txNonParallel, rollback)
			err := o.Transaction(ctx, input)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Transaction() err = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Transaction() err = nil, want %v", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Transaction() err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

/*
	Test cases for non parallel orchestrator:
	Non Parallel:
		- Success
		- Error on transaction with rollback
		- Error on transaction and rollback
*/

func TestNonParallel(t *testing.T) {
	tests := []struct {
		name             string
		txNonParallelErr error
		rollbackErr      error
		expectRollback   bool
		wantErr          error
	}{
		{
			name:             "Success",
			txNonParallelErr: nil,
			rollbackErr:      nil,
			expectRollback:   false,
			wantErr:          nil,
		},
		{
			name:             "Error on transaction with rollback",
			txNonParallelErr: fmt.Errorf("%w: tx failed", orchestrator.ErrorConsumerTransaction),
			rollbackErr:      nil,
			expectRollback:   true,
			wantErr:          nil,
		},
		{
			name:             "Error on transaction and rollback",
			txNonParallelErr: fmt.Errorf("%w: tx failed", orchestrator.ErrorConsumerTransaction),
			rollbackErr:      orchestrator.ErrorTransactionRollback,
			expectRollback:   true,
			wantErr:          orchestrator.ErrorTransactionRollback,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockstore.NewMockPsqlImpl(ctrl)

			// Transaction() persiste erro somente quando o erro final após rollback não é nil.
			if tc.rollbackErr != nil {
				store.EXPECT().
					SaveTransaction(gomock.Any(), gomock.Any(), tc.rollbackErr.Error()).
					Return(nil).
					Times(1)
			}

			input := orchestrator.Input{
				OrchestratorID: nonParallelRule.ID,
				TransactionID:  uuid.New(),
				Data:           map[string]string{"key": "value"},
				Headers:        map[string][]string{"X-Request-Id": {"req-123"}},
			}

			memStore := mockstore.NewMockMemoryImpl(ctrl)
			memStore.EXPECT().
				Find(gomock.Any(), nonParallelRule.ID).
				Return(nonParallelRule, nil).
				Times(1)

			txParallel := fakeTxUseCase{
				exec: func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error {
					t.Fatalf("transactionParallel.Execute called when rule.Configs.Parallel is false")
					return nil
				},
			}

			txNonParallel := fakeTxUseCase{
				exec: func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error {
					return tc.txNonParallelErr
				},
			}

			rollback := fakeRollbackUseCase{
				exec: func(ctx context.Context, transactions []rule.HTTPConfig, payload orchestrator.Input) error {
					if !tc.expectRollback {
						t.Fatalf("rollback.Execute called unexpectedly")
					}
					return tc.rollbackErr
				},
			}

			o := orchestrator.New(memStore, store, txParallel, txNonParallel, rollback)
			err := o.Transaction(ctx, input)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("Transaction() err = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Transaction() err = nil, want %v", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Transaction() err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
