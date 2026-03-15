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
		Handler: func(req *http.Request) connector.Response {
			paramID := req.PathValue("id")
			if paramID == "" {
				return connector.ResponseError{
					StatusCode: http.StatusBadRequest,
					Body: connector.DataErr{
						Msg:    "Orchestrator ID is required",
						Action: "please create the rule first",
					},
				}
			}

			ruleID, err := uuid.Parse(paramID)
			if err != nil {
				return connector.ResponseError{
					StatusCode: http.StatusBadRequest,
					Body: connector.DataErr{
						Msg:    "Invalid orchestrator ID",
						Action: "please create the rule first",
					},
				}
			}

			var data map[string]any
			if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
				return connector.ResponseError{
					StatusCode: http.StatusBadRequest,
					Body: connector.DataErr{
						Msg:    "Invalid request body",
						Action: "please check the request body",
					},
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

			switch {
			case errors.Is(err, orchestrator.ErrorRuleNotFound):
				return connector.ResponseError{
					StatusCode: http.StatusNotFound,
					Body: connector.DataErr{
						Msg:    "Rule not found",
						Action: "please create the rule first",
					},
				}
			case errors.Is(err, orchestrator.ErrorTransactionRollback):
				return connector.ResponseError{
					StatusCode: http.StatusFailedDependency,
					Body: connector.DataErr{
						Msg:    "Error when transaction rollback, inconsistent state",
						Action: "you have rollback some transactions with id: " + txID,
					},
				}
			case errors.Is(err, orchestrator.ErrorTransactionFailed):
				return connector.ResponseError{
					StatusCode: http.StatusFailedDependency,
					Body: connector.DataErr{
						Msg:    "Error transaction failed, rollback successful",
						Action: "please try again",
					},
				}
			}

			if err != nil {
				// TODO: métrica de erro interno não tratado
				return connector.ResponseError{
					StatusCode: http.StatusInternalServerError,
					Body: connector.DataErr{
						Msg:    "Error executing transaction, inconsistent state",
						Action: "please try again later",
					},
					Headers: http.Header{
						HeaderTransactionID: []string{txID},
					},
				}
			}

			return connector.ResponseOK{
				StatusCode: http.StatusAccepted,
				Body: map[string]string{
					"message": "Transaction accepted",
					"tx_id":   txID,
				},
				Headers: http.Header{
					HeaderTransactionID: []string{txID},
					"Content-Type":      []string{"application/json"},
					"Cache-Control":     []string{"no-cache, no-store"},
				},
			}
		},
	}
}
