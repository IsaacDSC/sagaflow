package putrule

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type Orchestrator interface {
	UpsertRule(ctx context.Context, rl rule.Rule) (rule.Rule, error)
}

// Handler registers PUT /api/v1/rule. Persistence and gqueue registration run in Orchestrator.UpsertRule.
func Handler(o Orchestrator) connector.Handler {
	return connector.Handler{
		Path: "PUT /api/v1/rule",
		Handler: func(req *http.Request) connector.Response {
			var rl rule.Rule
			if err := json.NewDecoder(req.Body).Decode(&rl); err != nil {
				return connector.ResponseError{
					StatusCode: http.StatusBadRequest,
					Body: connector.DataErr{
						Msg:    "Invalid request body",
						Action: "please check the request body",
					},
				}
			}

			if err := rl.Validate(); err != nil {
				return connector.ResponseError{
					StatusCode: http.StatusBadRequest,
					Body: connector.DataErr{
						Msg:    err.Error(),
						Action: "please check the request body",
					},
				}
			}

			saved, err := o.UpsertRule(req.Context(), rl)
			if err != nil {
				if errors.Is(err, orchestrator.ErrorAsyncNotConfigured) {
					return connector.ResponseError{
						StatusCode: http.StatusServiceUnavailable,
						Body: connector.DataErr{
							Msg:    "async rules require gqueue; configure GQUEUE_BASE_URL",
							Action: "set GQUEUE_BASE_URL and restart, or use sync mode (configs.sync: true)",
						},
					}
				}
				if errors.Is(err, orchestrator.ErrorSaveRule) {
					logger.Error(req.Context(), "error creating rule", "error", err)
					return connector.ResponseError{
						StatusCode: http.StatusInternalServerError,
						Body: connector.DataErr{
							Msg:    "Error creating rule",
							Action: "please try again later",
						},
					}
				}
				if errors.Is(err, orchestrator.ErrorGqueueUpsertRule) {
					logger.Error(req.Context(), "gqueue upsert event consumer failed", "error", err, "rule", rl.Name)
					return connector.ResponseError{
						StatusCode: http.StatusBadGateway,
						Body: connector.DataErr{
							Msg:    "Failed to register rule in gqueue",
							Action: "check gqueue availability and credentials, then retry",
						},
					}
				}
				logger.Error(req.Context(), "upsert rule failed", "error", err, "rule", rl.Name)
				return connector.ResponseError{
					StatusCode: http.StatusInternalServerError,
					Body: connector.DataErr{
						Msg:    "Error creating rule",
						Action: "please try again later",
					},
				}
			}

			return connector.ResponseOK{
				StatusCode: http.StatusOK,
				Body:       saved,
			}
		},
	}
}
