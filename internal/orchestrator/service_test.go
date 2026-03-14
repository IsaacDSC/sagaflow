package orchestrator_test

import (
	"context"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/mocks/mockorchestrator"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestExecute_Parallel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestratorID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	r := rule.Rule{
		ID:   orchestratorID,
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

	store := mockorchestrator.NewMockStore(ctrl)
	queue := mockorchestrator.NewMockQueue(ctrl)

	store.EXPECT().
		Find(gomock.Any(), orchestratorID).
		Return(r, nil).Times(1)
	queue.EXPECT().
		Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	queue.EXPECT().
		Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	o := orchestrator.New(store, queue)
	ctx := context.Background()
	input := orchestrator.Input{
		OrchestratorID: orchestratorID,
		Data:           map[string]string{"key": "value"},
	}

	err := o.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() err = %v, want nil", err)
	}
}

func TestExecute_NonParallel(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	orchestratorID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")
	r := rule.Rule{
		ID:   orchestratorID,
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

	store := mockorchestrator.NewMockStore(ctrl)
	queue := mockorchestrator.NewMockQueue(ctrl)

	store.EXPECT().
		Find(gomock.Any(), orchestratorID).
		Return(r, nil).Times(1)
	gomock.InOrder(
		queue.EXPECT().
			Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc1/do"}, gomock.Any(), gomock.Any()).
			Return(nil).Times(1),
		queue.EXPECT().
			Send(gomock.Any(), rule.HTTPConfig{Method: "POST", URL: "http://svc2/do"}, gomock.Any(), gomock.Any()).
			Return(nil).Times(1),
	)

	o := orchestrator.New(store, queue)
	ctx := context.Background()
	input := orchestrator.Input{
		OrchestratorID: orchestratorID,
		Data:           map[string]string{"key": "value"},
	}

	err := o.Execute(ctx, input)
	if err != nil {
		t.Fatalf("Execute() err = %v, want nil", err)
	}
}
