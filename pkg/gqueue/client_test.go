package gqueue

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient_requiresBaseURL(t *testing.T) {
	t.Parallel()
	if _, err := NewClient(); err == nil {
		t.Fatal("want error")
	}
	if _, err := NewClient(WithHTTPClient(http.DefaultClient)); err == nil {
		t.Fatal("want error")
	}
}

func TestClient_UpsertEventConsumer(t *testing.T) {
	t.Parallel()
	var gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/event/consumer" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithBasicAuth("admin", "password"),
	)
	if err != nil {
		t.Fatal(err)
	}
	in := UpsertEventConsumerInput{
		Name: "payment.charged",
		Type: "external",
		Option: EventOption{
			WQType:     "low_throughput",
			MaxRetries: 3,
			Retention:  "168h",
		},
		Consumers: []Consumer{{
			ServiceName: "external-service",
			Type:        "persistent",
			Host:        "http://consumer:3333",
			Path:        "/payment/charged",
			Headers:     map[string]string{"Content-Type": "application/json"},
		}},
	}
	if err := c.UpsertEventConsumer(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	wantTok := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:password"))
	if gotAuth != wantTok {
		t.Fatalf("Authorization: got %q want %q", gotAuth, wantTok)
	}
	var decoded UpsertEventConsumerInput
	if err := json.Unmarshal(gotBody, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Name != in.Name || decoded.Consumers[0].Host != in.Consumers[0].Host {
		t.Fatalf("%+v", decoded)
	}
}

func TestClient_Publish(t *testing.T) {
	t.Parallel()
	var gotAuth string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/pubsub" {
			http.NotFound(w, r)
			return
		}
		gotAuth = r.Header.Get("Authorization")
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(
		WithBaseURL(srv.URL),
		WithHTTPClient(srv.Client()),
		WithBasicAuth("admin", "password"),
	)
	if err != nil {
		t.Fatal(err)
	}
	in := PublishInput{
		ServiceName: "my-app",
		EventName:   "payment.charged",
		Data:        map[string]any{"key": "value", "w-queue": "pubsub"},
		Metadata:    map[string]any{"correlation_id": "5e4dd662-9eba-4321-9c97-0b4ee0942f8b"},
	}
	if err := c.Publish(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Fatalf("missing basic auth: %q", gotAuth)
	}
	var got PublishInput
	if err := json.Unmarshal(gotBody, &got); err != nil {
		t.Fatal(err)
	}
	if got.EventName != in.EventName || got.ServiceName != in.ServiceName {
		t.Fatalf("%+v", got)
	}
}

func TestClient_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad"}`))
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Publish(context.Background(), PublishInput{
		ServiceName: "s",
		EventName:   "e",
		Data:        map[string]any{},
	})
	if err == nil {
		t.Fatal("want API error")
	}
	if !IsAPIError(err) {
		t.Fatalf("IsAPIError: %v", err)
	}
}
