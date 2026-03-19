package entry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/entry"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/mocks/mockentry"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestHandler(t *testing.T) {
	validID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	validBody := map[string]any{"key": "value"}
	validBodyBytes, _ := json.Marshal(validBody)

	tests := []struct {
		name            string
		pathID          string
		body            []byte
		setupMock       func(*mockentry.MockOrchestrator)
		wantStatusCode  int
		wantBody        connector.DataErr
		wantActionPrefix string
		wantOK          bool
		wantOKBody      map[string]string
	}{
		{
			name:   "error parsing ID",
			pathID: "not-a-valid-uuid",
			body:   validBodyBytes,
			setupMock: func(m *mockentry.MockOrchestrator) {
				m.EXPECT().Transaction(gomock.Any(), gomock.Any()).Times(0)
			},
			wantStatusCode: http.StatusBadRequest,
			wantBody: connector.DataErr{
				Msg:    "Invalid orchestrator ID",
				Action: "please create the rule first",
			},
		},
		{
			name:   "ErrorRuleNotFound",
			pathID: validID.String(),
			body:   validBodyBytes,
			setupMock: func(m *mockentry.MockOrchestrator) {
				m.EXPECT().
					Transaction(gomock.Any(), gomock.Any()).
					Return(orchestrator.ErrorRuleNotFound).
					Times(1)
			},
			wantStatusCode: http.StatusNotFound,
			wantBody: connector.DataErr{
				Msg:    "Rule not found",
				Action: "please create the rule first",
			},
		},
		{
			name:   "ErrorTransactionRollback",
			pathID: validID.String(),
			body:   validBodyBytes,
			setupMock: func(m *mockentry.MockOrchestrator) {
				m.EXPECT().
					Transaction(gomock.Any(), gomock.Any()).
					Return(orchestrator.ErrorTransactionRollback).
					Times(1)
			},
			wantStatusCode:   http.StatusFailedDependency,
			wantBody:         connector.DataErr{Msg: "Error when transaction rollback, temporary inconsistent state"},
			wantActionPrefix: "you have rollback some transactions with id: ",
		},
		{
			name:   "ErrorTransactionFailed",
			pathID: validID.String(),
			body:   validBodyBytes,
			setupMock: func(m *mockentry.MockOrchestrator) {
				m.EXPECT().
					Transaction(gomock.Any(), gomock.Any()).
					Return(orchestrator.ErrorTransactionFailed).
					Times(1)
			},
			wantStatusCode: http.StatusFailedDependency,
			wantBody: connector.DataErr{
				Msg:    "Error transaction failed, rollback successful",
				Action: "please try again",
			},
		},
		{
			name:   "generic error",
			pathID: validID.String(),
			body:   validBodyBytes,
			setupMock: func(m *mockentry.MockOrchestrator) {
				m.EXPECT().
					Transaction(gomock.Any(), gomock.Any()).
					Return(errors.New("internal error")).
					Times(1)
			},
			wantStatusCode: http.StatusInternalServerError,
			wantBody: connector.DataErr{
				Msg:    "Error executing transaction, inconsistent state",
				Action: "please try again later",
			},
		},
		{
			name:   "success",
			pathID: validID.String(),
			body:   validBodyBytes,
			setupMock: func(m *mockentry.MockOrchestrator) {
				m.EXPECT().
					Transaction(gomock.Any(), gomock.Any()).
					DoAndReturn(func(ctx context.Context, input orchestrator.Input) error {
						if input.OrchestratorID != validID {
							t.Errorf("Transaction OrchestratorID = %v, want %v", input.OrchestratorID, validID)
						}
						return nil
					}).
					Times(1)
			},
			wantOK:     true,
			wantOKBody: map[string]string{"message": "Transaction accepted"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockOrch := mockentry.NewMockOrchestrator(ctrl)
			tt.setupMock(mockOrch)

			h := entry.Handler(mockOrch)
			req := newRequest(t, tt.pathID, tt.body)

			resp := h.Handler(req)

			if tt.wantOK {
				if resp.Code() != http.StatusAccepted {
					t.Errorf("StatusCode = %d, want %d", resp.Code(), http.StatusAccepted)
				}
				data, ok := resp.Data().(map[string]string)
				if !ok {
					t.Fatalf("Body type = %T, want map[string]string", resp.Data())
				}
				if data["message"] != tt.wantOKBody["message"] {
					t.Errorf("Body.message = %q, want %q", data["message"], tt.wantOKBody["message"])
				}
				if data["tx_id"] == "" {
					t.Error("Body.tx_id should not be empty")
				}
				return
			}

			if resp.Code() != tt.wantStatusCode {
				t.Errorf("StatusCode = %d, want %d", resp.Code(), tt.wantStatusCode)
			}
			bodyErr, ok := resp.Data().(connector.DataErr)
			if !ok {
				t.Fatalf("Body type = %T, want connector.DataErr", resp.Data())
			}
			if bodyErr.Msg != tt.wantBody.Msg {
				t.Errorf("Body.Msg = %q, want %q", bodyErr.Msg, tt.wantBody.Msg)
			}
			if tt.wantActionPrefix != "" {
				if !strings.HasPrefix(bodyErr.Action, tt.wantActionPrefix) {
					t.Errorf("Body.Action = %q, want prefix %q", bodyErr.Action, tt.wantActionPrefix)
				}
			} else if bodyErr.Action != tt.wantBody.Action {
				t.Errorf("Body.Action = %q, want %q", bodyErr.Action, tt.wantBody.Action)
			}
		})
	}
}

func newRequest(t *testing.T, pathID string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "/api/v1/transaction/rule/"+pathID, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.SetPathValue("id", pathID)
	return req
}
