package nofifygate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/google/uuid"
)

func TestHttpClient_Send_Success(t *testing.T) {
	t.Parallel()

	var (
		gotMethod string
		gotPath   string
		gotBody   []byte
		gotHeader http.Header
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		_ = r.Body.Close()
		gotBody = body
		gotHeader = r.Header.Clone()

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL + "/test-endpoint")
	if err != nil {
		t.Fatalf("failed to parse test url: %v", err)
	}

	httpCfg := rule.HTTPConfig{
		Method: http.MethodPost,
		URL:    u.String(),
	}

	orchID := uuid.New()
	input := orchestrator.Input{
		OrchestratorID: orchID,
		Data: map[string]string{
			"foo": "bar",
		},
		Headers: map[string][]string{
			"X-Custom": {"value-1", "value-2"},
		},
	}

	conf := rule.Configs{
		MaxTimeout: "1s",
		MaxRetry:   0,
	}

	client := NewHttpClient()
	if err := client.Send(context.Background(), httpCfg, input, conf); err != nil {
		t.Fatalf("Send() unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected method %q, got %q", http.MethodPost, gotMethod)
	}

	if gotPath != "/test-endpoint" && gotPath != "/test-endpoint/" {
		t.Fatalf("expected path %q or %q, got %q", "/test-endpoint", "/test-endpoint/", gotPath)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if payload["foo"] != "bar" {
		t.Fatalf("expected payload foo=%q, got %q", "bar", payload["foo"])
	}

	if got := gotHeader.Get("x-transaction-id"); got != orchID.String() {
		t.Fatalf("expected x-transaction-id %q, got %q", orchID.String(), got)
	}

	if got := gotHeader.Get("X-Custom"); got != "value-1" {
		t.Fatalf("expected X-Custom header %q, got %q", "value-1", got)
	}

	if got := gotHeader.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type %q, got %q", "application/json", got)
	}
}

func TestHttpClient_Send_ErrorStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL + "/test-endpoint")
	if err != nil {
		t.Fatalf("failed to parse test url: %v", err)
	}

	httpCfg := rule.HTTPConfig{
		Method: http.MethodPost,
		URL:    u.String(),
	}

	input := orchestrator.Input{
		OrchestratorID: uuid.New(),
		Data:           nil,
		Headers:        nil,
	}

	conf := rule.Configs{
		MaxTimeout: "1s",
		MaxRetry:   0,
	}

	client := NewHttpClient()
	if err := client.Send(context.Background(), httpCfg, input, conf); err == nil {
		t.Fatalf("expected error for non-2xx response, got nil")
	}
}
