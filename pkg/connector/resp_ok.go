package connector

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type ResponseOK struct {
	StatusCode int
	Headers    http.Header
	Body       any
}

var _ Response = &ResponseOK{}

func (r ResponseOK) Write(w http.ResponseWriter) error {
	for key, values := range r.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(r.StatusCode)
	if r.Body != nil {
		return json.NewEncoder(w).Encode(r.Body)
	}

	return nil
}

func (r ResponseOK) Code() int {
	return r.StatusCode
}

func (r ResponseOK) BodyBytes() []byte {
	body, err := json.Marshal(r.Body)
	if err != nil {
		logger.Error(context.Background(), "error marshalling response body", "error", err, "tag", "ResponseOK.BodyBytes")
		return nil
	}
	return body
}

func (r ResponseOK) Data() any {
	return r.Body
}
