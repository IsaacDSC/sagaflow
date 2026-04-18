package putrule_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/putrule"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/mocks/mockgqueue"
	"github.com/IsaacDSC/sagaflow/mocks/mockputrule"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/IsaacDSC/sagaflow/pkg/gqueue"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestHandler_InvalidBody(t *testing.T) {
	h := putrule.Handler(nil)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", []byte("not json"))

	resp := h.Handler(req)

	if resp.Code() != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", resp.Code(), http.StatusBadRequest)
	}
	expectedBody := connector.DataErr{
		Msg:    "Invalid request body",
		Action: "please check the request body",
	}

	if resp.Data() != expectedBody {
		t.Errorf("Body = %v, want %v", resp.Data(), expectedBody)
	}
}

func TestHandler_SaveError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockputrule.NewMockStore(ctrl)
	store.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(uuid.Nil, errors.New("db error")).
		Times(1)

	body := rule.Rule{
		Name: "test-rule",
		Transactions: []rule.HTTPConfig{{
			ServiceName: "orders",
			Method:      "POST",
			URL:         "http://svc/do",
		}},
		Rollback: []rule.HTTPConfig{{
			ServiceName: "orders",
			Method:      "POST",
			URL:         "http://svc/undo",
		}},
		Configs: rule.Configs{Sync: true},
	}
	raw, _ := json.Marshal(body)

	orch := orchestrator.New(nil, nil, nil, nil, nil, nil, nil, store)
	h := putrule.Handler(orch)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", raw)

	resp := h.Handler(req)

	if resp.Code() != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", resp.Code(), http.StatusInternalServerError)
	}
	bodyErr, ok := resp.Data().(connector.DataErr)
	if !ok {
		t.Fatalf("Body type = %T, want connector.BodyErr", resp.Data())
	}
	if bodyErr.Msg != "Error creating rule" {
		t.Errorf("Body.Msg = %q, want %q", bodyErr.Msg, "Error creating rule")
	}
}

func TestHandler_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	store := mockputrule.NewMockStore(ctrl)
	store.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, r rule.Rule) (uuid.UUID, error) {
			if r.Name != "my-saga" {
				t.Errorf("Save rule.Name = %q, want %q", r.Name, "my-saga")
			}
			return id, nil
		}).
		Times(1)

	body := rule.Rule{
		Name: "my-saga",
		Transactions: []rule.HTTPConfig{{
			ServiceName: "orders",
			Method:      "POST",
			URL:         "http://svc/do",
		}},
		Rollback: []rule.HTTPConfig{{
			ServiceName: "orders",
			Method:      "POST",
			URL:         "http://svc/undo",
		}},
		Configs: rule.Configs{Sync: true},
	}
	raw, _ := json.Marshal(body)

	orch := orchestrator.New(nil, nil, nil, nil, nil, nil, nil, store)
	h := putrule.Handler(orch)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", raw)

	resp := h.Handler(req)

	if resp.Code() != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.Code(), http.StatusOK)
	}
	rl, ok := resp.Data().(rule.Rule)
	if !ok {
		t.Fatalf("Body type = %T, want rule.Rule", resp.Data())
	}
	if rl.ID != id {
		t.Errorf("Body.ID = %v, want %v", rl.ID, id)
	}
	if rl.Name != "my-saga" {
		t.Errorf("Body.Name = %q, want %q", rl.Name, "my-saga")
	}
}

func TestHandler_AsyncRequiresGqueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockputrule.NewMockStore(ctrl)
	store.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	body := rule.Rule{
		Name: "async-rule",
		Transactions: []rule.HTTPConfig{{
			ServiceName: "orders",
			Method:      "POST",
			URL:         "http://localhost:3002/order",
		}},
		Configs: rule.Configs{Sync: false},
	}
	raw, _ := json.Marshal(body)

	orch := orchestrator.New(nil, nil, nil, nil, nil, nil, nil, store)
	h := putrule.Handler(orch)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", raw)

	resp := h.Handler(req)
	if resp.Code() != http.StatusServiceUnavailable {
		t.Errorf("StatusCode = %d, want %d", resp.Code(), http.StatusServiceUnavailable)
	}
}

func TestHandler_AsyncUpsertsGqueue(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	store := mockputrule.NewMockStore(ctrl)
	store.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(id, nil).
		Times(1)

	gq := mockgqueue.NewMockAPI(ctrl)
	gq.EXPECT().
		UpsertEventConsumer(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, in gqueue.UpsertEventConsumerInput) error {
			if in.Name != "async-rule" {
				t.Errorf("UpsertEventConsumer name = %q, want %q", in.Name, "async-rule")
			}
			if in.Type != "external" {
				t.Errorf("UpsertEventConsumer type = %q, want %q", in.Type, "external")
			}
			if len(in.Consumers) != 1 {
				t.Fatalf("Consumers len = %d, want 1", len(in.Consumers))
			}
			c := in.Consumers[0]
			if c.Host != "http://localhost:3002" || c.Path != "/order" {
				t.Errorf("Consumer host/path = %q %q, want http://localhost:3002 /order", c.Host, c.Path)
			}
			return nil
		}).
		Times(1)

	orch := orchestrator.New(nil, nil, nil, nil, nil, nil, gq, store)

	body := rule.Rule{
		Name: "async-rule",
		Transactions: []rule.HTTPConfig{{
			ServiceName: "orders",
			Method:      "POST",
			URL:         "http://localhost:3002/order",
		}},
		Configs: rule.Configs{Sync: false},
	}
	raw, _ := json.Marshal(body)

	h := putrule.Handler(orch)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", raw)

	resp := h.Handler(req)
	if resp.Code() != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.Code(), http.StatusOK)
	}
}

func mustNewRequest(t *testing.T, method, path string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, path, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req
}
