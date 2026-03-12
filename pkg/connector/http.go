package connector

import (
	"encoding/json"
	"net/http"
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

		resp := fn(r)

		if err := resp.Write(w); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal server error"))
		}
	}
}
