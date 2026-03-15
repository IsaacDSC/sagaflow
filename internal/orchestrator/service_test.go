package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/mocks/mockorchestrator"
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

func TestParallel(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue)
		wantErr error
	}{
		{
			name: "Success",
			setup: func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue) {
				store.EXPECT().Find(gomock.Any(), parallelRule.ID).Return(parallelRule, nil).Times(1)
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			wantErr: nil,
		},
		{
			name: "Error on transaction with rollback",
			setup: func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue) {
				store.EXPECT().Find(gomock.Any(), parallelRule.ID).Return(parallelRule, nil).Times(1)
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).Return(errors.New("tx failed")).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/undo"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/undo"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			},
			wantErr: orchestrator.ErrorTransactionFailed,
		},
		{
			name: "Error on transaction and rollback",
			setup: func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue) {
				store.EXPECT().Find(gomock.Any(), parallelRule.ID).Return(parallelRule, nil).Times(1)
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).Return(errors.New("tx failed")).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/undo"}, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/undo"}, gomock.Any(), gomock.Any()).Return(errors.New("rollback failed")).AnyTimes()
			},
			wantErr: orchestrator.ErrorTransactionRollback,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockorchestrator.NewMockStore(ctrl)
			queue := mockorchestrator.NewMockQueue(ctrl)
			tc.setup(store, queue)
			o := orchestrator.New(store, queue)
			ctx := context.Background()
			input := orchestrator.Input{OrchestratorID: parallelRule.ID, Data: map[string]string{"key": "value"}}
			err := o.Execute(ctx, input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Execute() err = %v, want %v", err, tc.wantErr)
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
		name    string
		setup   func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue)
		wantErr error
	}{
		{
			name: "Success",
			setup: func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue) {
				store.EXPECT().Find(gomock.Any(), nonParallelRule.ID).Return(nonParallelRule, nil).Times(1)
				gomock.InOrder(
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).Return(nil).Times(1),
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).Return(nil).Times(1),
				)
			},
			wantErr: nil,
		},
		{
			name: "Error on transaction with rollback",
			setup: func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue) {
				store.EXPECT().Find(gomock.Any(), nonParallelRule.ID).Return(nonParallelRule, nil).Times(1)
				gomock.InOrder(
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).Return(nil).Times(1),
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).Return(errors.New("tx failed")).Times(1),
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/undo"}, gomock.Any(), gomock.Any()).Return(nil).Times(1),
				)
			},
			wantErr: orchestrator.ErrorTransactionFailed,
		},
		{
			name: "Error on transaction and rollback",
			setup: func(store *mockorchestrator.MockStore, queue *mockorchestrator.MockQueue) {
				store.EXPECT().Find(gomock.Any(), nonParallelRule.ID).Return(nonParallelRule, nil).Times(1)
				gomock.InOrder(
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).Return(nil).Times(1),
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).Return(errors.New("tx failed")).Times(1),
					queue.EXPECT().Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/undo"}, gomock.Any(), gomock.Any()).Return(errors.New("rollback failed")).Times(1),
				)
			},
			wantErr: orchestrator.ErrorTransactionRollback,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockorchestrator.NewMockStore(ctrl)
			queue := mockorchestrator.NewMockQueue(ctrl)
			tc.setup(store, queue)
			o := orchestrator.New(store, queue)
			ctx := context.Background()
			input := orchestrator.Input{OrchestratorID: nonParallelRule.ID, Data: map[string]string{"key": "value"}}
			err := o.Execute(ctx, input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Execute() err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
