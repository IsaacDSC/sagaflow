package orchestrator_test

import (
	"context"
	"errors"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/mocks/mockgqueue"
	"github.com/IsaacDSC/sagaflow/pkg/gqueue"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestTransactionAsync_Execute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	api := mockgqueue.NewMockAPI(ctrl)

	payload := orchestrator.Input{
		TransactionID:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		OrchestratorID: uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Data:           map[string]any{"k": "v"},
	}

	api.EXPECT().Publish(gomock.Any(), gomock.AssignableToTypeOf(gqueue.PublishInput{})).
		Times(1).
		DoAndReturn(func(ctx context.Context, in gqueue.PublishInput) error {
			if in.ServiceName != "sagaflow:rule:my-rule" {
				t.Fatalf("ServiceName = %q", in.ServiceName)
			}
			if in.Data["payload"] == nil {
				t.Fatal("expected payload in data")
			}
			return nil
		})

	svc := orchestrator.NewTransactionAsync(api, "sagaflow-test")
	if err := svc.Execute(ctx, "my-rule", payload, rule.Configs{Parallel: false}); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionAsync_Execute_parallelConfigsStillSinglePublish(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	api := mockgqueue.NewMockAPI(ctrl)

	payload := orchestrator.Input{
		TransactionID:  uuid.New(),
		OrchestratorID: uuid.New(),
		Data:           nil,
	}

	api.EXPECT().Publish(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	svc := orchestrator.NewTransactionAsync(api, "")
	if err := svc.Execute(ctx, "rule-a", payload, rule.Configs{Parallel: true}); err != nil {
		t.Fatal(err)
	}
}

func TestTransactionAsync_Execute_nilQueue(t *testing.T) {
	t.Parallel()
	svc := orchestrator.NewTransactionAsync(nil, "")
	err := svc.Execute(context.Background(), "x", orchestrator.Input{}, rule.Configs{})
	if err == nil {
		t.Fatal("want error")
	}
	if !errors.Is(err, orchestrator.ErrorAsyncNotConfigured) {
		t.Fatalf("got %v", err)
	}
}
