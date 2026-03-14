package putrule_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/putrule"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/mocks/mockputrule"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestHandler_InvalidBody(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mockputrule.NewMockStore(ctrl)
	store.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	h := putrule.Handler(store)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", []byte("not json"))

	resp := h.Handler(req)

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if resp.Body != "Invalid request body" {
		t.Errorf("Body = %v, want %q", resp.Body, "Invalid request body")
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
		Transactions: []rule.HTTPConfig{{Method: "POST", URL: "http://svc/do"}},
		Rollback:     []rule.HTTPConfig{{Method: "POST", URL: "http://svc/undo"}},
	}
	raw, _ := json.Marshal(body)

	h := putrule.Handler(store)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", raw)

	resp := h.Handler(req)

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}
	if resp.Body != "Error creating rule" {
		t.Errorf("Body = %v, want %q", resp.Body, "Error creating rule")
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
		Transactions: []rule.HTTPConfig{{Method: "POST", URL: "http://svc/do"}},
		Rollback:     []rule.HTTPConfig{{Method: "POST", URL: "http://svc/undo"}},
	}
	raw, _ := json.Marshal(body)

	h := putrule.Handler(store)
	req := mustNewRequest(t, http.MethodPut, "/api/v1/rule", raw)

	resp := h.Handler(req)

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	rl, ok := resp.Body.(rule.Rule)
	if !ok {
		t.Fatalf("Body type = %T, want rule.Rule", resp.Body)
	}
	if rl.ID != id {
		t.Errorf("Body.ID = %v, want %v", rl.ID, id)
	}
	if rl.Name != "my-saga" {
		t.Errorf("Body.Name = %q, want %q", rl.Name, "my-saga")
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
