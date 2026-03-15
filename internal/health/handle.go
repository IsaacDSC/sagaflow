package health

import (
	"net/http"

	"github.com/IsaacDSC/sagaflow/pkg/connector"
)

func Handler() connector.Handler {
	return connector.Handler{
		Path: "GET /ping",
		Handler: func(req *http.Request) connector.Response {
			return connector.ResponseOK{
				StatusCode: http.StatusOK,
				Body:       "pong",
			}
		},
	}
}
