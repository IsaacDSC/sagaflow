package connector

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type ResponseError struct {
	StatusCode int
	Headers    http.Header
	Body       DataErr
}

var _ Response = &ResponseError{}

type DataErr struct {
	Msg    string `json:"msg"`
	Action string `json:"action"`
}

func (r ResponseError) Write(w http.ResponseWriter) error {
	for key, values := range r.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(r.StatusCode)
	err := json.NewEncoder(w).Encode(r.Body)
	if err != nil {
		logger.Error(context.Background(), "error encoding response", "error", err, "tag", "ResponseError.Write")
		return err
	}

	return nil
}

func (r ResponseError) Code() int {
	return r.StatusCode
}

func (r ResponseError) BodyBytes() []byte {
	body, err := json.Marshal(r.Body)
	if err != nil {
		logger.Error(context.Background(), "error marshalling response body", "error", err, "tag", "ResponseError.BodyBytes")
		return nil
	}
	return body
}

func (r ResponseError) Data() any {
	return r.Body
}
