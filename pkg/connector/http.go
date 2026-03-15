package connector

import (
	"net/http"

	"github.com/IsaacDSC/sagaflow/pkg/logger"
)

type Handler struct {
	Path    string
	Handler func(req *http.Request) Response
}

type Response interface {
	Write(w http.ResponseWriter) error
	Code() int
	BodyBytes() []byte
	Data() any
}

func Adapter(fn func(req *http.Request) Response) http.HandlerFunc {
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
		case resp.Code() >= 200 && resp.Code() >= 400:
			reqLogger.Info("request successful", "status", resp.Code())
		case resp.Code() >= 400 && resp.Code() < 500:
			reqLogger.Warn("request failed", "status", resp.Code(), "error", string(resp.BodyBytes()))
		case resp.Code() >= 500:
			reqLogger.Error("request failed", "status", resp.Code(), "error", string(resp.BodyBytes()))
		}

		if err := resp.Write(w); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal server error"))
		}
	}
}
