package connector

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponse_Write(t *testing.T) {
	t.Run("writes status code and JSON body", func(t *testing.T) {
		body := map[string]string{"key": "value"}
		resp := Response{
			StatusCode: http.StatusOK,
			Body:       body,
			Headers:    http.Header{},
		}
		w := httptest.NewRecorder()

		err := resp.Write(w)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var got map[string]string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got["key"] != "value" {
			t.Errorf("body = %v, want key=value", got)
		}
	})

	t.Run("writes custom headers", func(t *testing.T) {
		resp := Response{
			StatusCode: http.StatusCreated,
			Body:       nil,
			Headers: http.Header{
				"X-Request-Id": {"req-123"},
				"Cache-Control": {"no-cache"},
			},
		}
		w := httptest.NewRecorder()

		err := resp.Write(w)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
		}
		if id := w.Header().Get("X-Request-Id"); id != "req-123" {
			t.Errorf("X-Request-Id = %q, want req-123", id)
		}
		if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
			t.Errorf("Cache-Control = %q, want no-cache", cc)
		}
	})

	t.Run("writes nil body without error", func(t *testing.T) {
		resp := Response{
			StatusCode: http.StatusNoContent,
			Body:       nil,
			Headers:    http.Header{},
		}
		w := httptest.NewRecorder()

		err := resp.Write(w)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}

		if w.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
		}
		if w.Body.Len() != 0 {
			t.Errorf("body len = %d, want 0", w.Body.Len())
		}
	})
}

func TestAdapter(t *testing.T) {
	t.Run("sets Content-Type and calls handler", func(t *testing.T) {
		handler := Adapter(func(req *http.Request) *Response {
			return &Response{
				StatusCode: http.StatusOK,
				Body:       map[string]string{"ok": "true"},
				Headers:    http.Header{},
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		var got map[string]string
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got["ok"] != "true" {
			t.Errorf("body = %v, want ok=true", got)
		}
	})

	t.Run("passes request with context to handler", func(t *testing.T) {
		var receivedReq *http.Request
		handler := Adapter(func(req *http.Request) *Response {
			receivedReq = req
			return &Response{StatusCode: http.StatusOK, Headers: http.Header{}}
		})

		req := httptest.NewRequest(http.MethodPost, "/echo", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		if receivedReq == nil {
			t.Fatal("handler was not called")
		}
		if receivedReq.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", receivedReq.Method)
		}
		if receivedReq.URL.Path != "/echo" {
			t.Errorf("path = %s, want /echo", receivedReq.URL.Path)
		}
	})

	t.Run("writes response via Write", func(t *testing.T) {
		handler := Adapter(func(req *http.Request) *Response {
			return &Response{
				StatusCode: http.StatusCreated,
				Body:       map[string]int{"id": 42},
				Headers:    http.Header{"X-Custom": {"custom-value"}},
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
		}
		if x := w.Header().Get("X-Custom"); x != "custom-value" {
			t.Errorf("X-Custom = %q, want custom-value", x)
		}
		var got map[string]int
		if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if got["id"] != 42 {
			t.Errorf("body id = %v, want 42", got["id"])
		}
	})

	t.Run("writes Internal server error when Write fails", func(t *testing.T) {
		// Body that cannot be JSON-encoded causes resp.Write to return an error.
		// Status may already be 200 (written before Encode), but body must be overwritten.
		handler := Adapter(func(req *http.Request) *Response {
			return &Response{
				StatusCode: http.StatusOK,
				Body:       make(chan int), // cannot be JSON encoded
				Headers:    http.Header{},
			}
		})

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		if body := w.Body.String(); body != "Internal server error" {
			t.Errorf("body = %q, want %q", body, "Internal server error")
		}
		// Adapter attempts 500 when Write fails; if status was already sent, code stays 200
		if c := w.Code; c != http.StatusInternalServerError && c != http.StatusOK {
			t.Errorf("status = %d, want %d or %d", c, http.StatusInternalServerError, http.StatusOK)
		}
	})
}
