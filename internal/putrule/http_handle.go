package putrule

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	"github.com/google/uuid"
)

type (
	Store interface {
		Save(ctx context.Context, rule rule.Rule) (uuid.UUID, error)
	}
)

func Handler(s Store) connector.Handler {
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

			rl.ID = uuid.New()
			id, err := s.Save(req.Context(), rl)
			if err != nil {
				logger.Error(req.Context(), "error creating rule", "error", err)
				return connector.ResponseError{
					StatusCode: http.StatusInternalServerError,
					Body: connector.DataErr{
						Msg:    "Error creating rule",
						Action: "please try again later",
					},
				}
			}

			rl.ID = id

			return connector.ResponseOK{
				StatusCode: http.StatusOK,
				Body:       rl,
			}
		},
	}
}
