package entry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/google/uuid"
)

type (
	Orchestrator interface {
		Execute(ctx context.Context, txInput orchestrator.Input) error
	}
)

const (
	HeaderTransactionID = "x-transaction-id"
)

func Handler(o Orchestrator) connector.Handler {
	return connector.Handler{
		Path: "POST /api/v1/transaction/rule/{id}",
		Handler: func(req *http.Request) *connector.Response {
			paramID := req.PathValue("id")
			if paramID == "" {
				return &connector.Response{
					StatusCode: http.StatusBadRequest,
					Body:       "Orchestrator ID is required",
				}
			}

			ruleID, err := uuid.Parse(paramID)
			if err != nil {
				return &connector.Response{
					StatusCode: http.StatusBadRequest,
					Body:       "Invalid orchestrator ID",
				}
			}

			var data map[string]any
			if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
				return &connector.Response{
					StatusCode: http.StatusBadRequest,
					Body:       "Invalid request body",
				}
			}

			txID := uuid.New().String()
			req.Header.Add(HeaderTransactionID, txID)
			txInput := orchestrator.Input{
				OrchestratorID: ruleID,
				Data:           data,
				Headers:        req.Header.Clone(),
			}

			err = o.Execute(req.Context(), txInput)

			if errors.Is(err, orchestrator.ErrorRuleNotFound) {
				return &connector.Response{
					StatusCode: http.StatusNotFound,
					Body:       "Rule not found",
				}
			}

			if err != nil {
				return &connector.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       "Error executing transaction",
					Headers: http.Header{
						HeaderTransactionID: []string{txID},
					},
				}
			}

			return &connector.Response{
				StatusCode: http.StatusAccepted,
				Body:       "Transaction accepted",
				Headers: http.Header{
					HeaderTransactionID: []string{txID},
					"Content-Type":      []string{"application/json"},
					"Cache-Control":     []string{"no-cache, no-store"},
				},
			}
		},
	}
}
