package connector

import (
	"encoding/json"
	"net/http"

	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type Handler struct {
	Path    string
	Handler func(req *http.Request) *Response
}

type Response struct {
	StatusCode int
	Body       any
	Headers    http.Header
}

func (r Response) Write(w http.ResponseWriter) error {
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

func Adapter(fn func(req *http.Request) *Response) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		ctx := r.Context()
		reqLogger := logger.FromContext(ctx).With(
			"method", r.Method,
			"path", r.URL.Path,
		)

		ctx = logger.WithLogger(ctx, reqLogger)
		r = r.WithContext(ctx)

		resp := fn(r)

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 400:
			reqLogger.Info("request successful", "status", resp.StatusCode)
		case resp.StatusCode >= 400 && resp.StatusCode < 500:
			reqLogger.Warn("request failed", "status", resp.StatusCode, "error", resp.Body)
		case resp.StatusCode >= 500:
			reqLogger.Error("request failed", "status", resp.StatusCode, "error", resp.Body)
		}

		if err := resp.Write(w); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal server error"))
		}
	}
}
