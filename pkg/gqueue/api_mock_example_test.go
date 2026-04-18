package gqueue_test

import (
	"context"
	"testing"

	"github.com/IsaacDSC/sagaflow/mocks/mockgqueue"
	"github.com/IsaacDSC/sagaflow/pkg/gqueue"
	"go.uber.org/mock/gomock"
)

func publishCharge(ctx context.Context, api gqueue.API, in gqueue.PublishInput) error {
	return api.Publish(ctx, in)
}

func TestMockAPI_publishCharge(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	mock := mockgqueue.NewMockAPI(ctrl)

	in := gqueue.PublishInput{
		ServiceName: "my-app",
		EventName:   "payment.charged",
		Data:        map[string]any{"key": "value"},
	}
	mock.EXPECT().Publish(gomock.Any(), gomock.AssignableToTypeOf(gqueue.PublishInput{})).
		DoAndReturn(func(ctx context.Context, got gqueue.PublishInput) error {
			if got.ServiceName != in.ServiceName || got.EventName != in.EventName {
				t.Fatalf("unexpected input: %+v", got)
			}
			return nil
		})

	if err := publishCharge(context.Background(), mock, in); err != nil {
		t.Fatal(err)
	}
}
