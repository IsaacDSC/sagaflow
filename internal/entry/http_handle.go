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
		Transaction(ctx context.Context, txInput orchestrator.Input) (map[string]orchestrator.DataAggregator, error)
	}
)

const (
	HeaderTransactionID = "x-transaction-id"
)

type Response struct {
	Message       string                                 `json:"message"`
	TransactionID string                                 `json:"transaction_id"`
	Aggregator    map[string]orchestrator.DataAggregator `json:"aggregator,omitempty"`
}

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

			txID := uuid.New()
			req.Header.Add(HeaderTransactionID, txID.String())
			txInput := orchestrator.Input{
				TransactionID:  txID,
				OrchestratorID: ruleID,
				Data:           data,
				Headers:        req.Header.Clone(),
			}

			// if async
			// save transaction in database
			//  response ok with transaction id

			// if sync
			response, err := o.Transaction(req.Context(), txInput)

			switch {
			case errors.Is(err, orchestrator.ErrorAsyncNotConfigured):
				return connector.ResponseError{
					StatusCode: http.StatusServiceUnavailable,
					Body: connector.DataErr{
						Msg:    "Async mode requires gqueue",
						Action: "set GQUEUE_BASE_URL (and credentials) on the server",
					},
				}
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
						Msg:    "Error when transaction rollback, temporary inconsistent state",
						Action: "you have rollback some transactions with id: " + txID.String(),
					},
				}
			case errors.Is(err, orchestrator.ErrorSaveTransaction):
				return connector.ResponseError{
					StatusCode: http.StatusInternalServerError,
					Body: connector.DataErr{
						Msg:    "Error saving transaction, inconsistent state",
						Action: "please contact the administrator to fix the problem",
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
						HeaderTransactionID: []string{txID.String()},
					},
				}
			}

			return connector.ResponseOK{
				StatusCode: http.StatusAccepted,
				Body: Response{
					Message:       "Transaction accepted",
					TransactionID: txID.String(),
					Aggregator:    response,
				},
				Headers: http.Header{
					HeaderTransactionID: []string{txID.String()},
					"Content-Type":      []string{"application/json"},
					"Cache-Control":     []string{"no-cache, no-store"},
				},
			}
		},
	}
}
