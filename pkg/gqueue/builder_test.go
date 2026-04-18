package gqueue

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishBuilder_Build_ok(t *testing.T) {
	t.Parallel()
	b := NewPublishBuilder("my-app", "payment.charged").
		WithData(map[string]any{"key": "value", "w-queue": "pubsub"}).
		WithCorrelationID("5e4dd662-9eba-4321-9c97-0b4ee0942f8b")
	in, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	const wantSub = `"event_name":"payment.charged"`
	if !json.Valid(raw) {
		t.Fatal("invalid json")
	}
	s := string(raw)
	if !strings.Contains(s, wantSub) || !strings.Contains(s, `"correlation_id":"5e4dd662-9eba-4321-9c97-0b4ee0942f8b"`) {
		t.Fatalf("marshal: %s", s)
	}
}

func TestPublishBuilder_Build_validation(t *testing.T) {
	t.Parallel()
	_, err := NewPublishBuilder("", "e").Build()
	if err == nil {
		t.Fatal("want error")
	}
	_, err = NewPublishBuilder("svc", "").Build()
	if err == nil {
		t.Fatal("want error")
	}
}

func TestPublishBuilder_WithDataFrom(t *testing.T) {
	t.Parallel()
	type payload struct{ A int `json:"a"` }
	b := NewPublishBuilder("svc", "evt")
	b, err := b.WithDataFrom(payload{A: 42})
	if err != nil {
		t.Fatal(err)
	}
	in, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if in.Data["a"] != float64(42) { // JSON numbers decode as float64
		t.Fatalf("got %#v", in.Data["a"])
	}
}

func TestEventConsumerBuilder_Build_ok(t *testing.T) {
	t.Parallel()
	cb := NewConsumerBuilder().
		WithServiceName("external-service").
		WithConsumerType("persistent").
		WithHost("http://consumer:3333").
		WithPath("/payment/charged").
		WithHeader("Content-Type", "application/json")

	eb := NewEventConsumerBuilder("payment.charged").
		WithEventType("external").
		WithOption(EventOption{
			WQType:     "low_throughput",
			MaxRetries: 3,
			Retention:  "168h",
		})
	var err error
	eb, err = eb.AddConsumerFromBuilder(cb)
	if err != nil {
		t.Fatal(err)
	}
	in, err := eb.Build()
	if err != nil {
		t.Fatal(err)
	}
	if in.Name != "payment.charged" || in.Type != "external" {
		t.Fatalf("%+v", in)
	}
	if len(in.Consumers) != 1 || in.Consumers[0].ServiceName != "external-service" {
		t.Fatalf("%+v", in)
	}
}

func TestEventConsumerBuilder_Build_requiresConsumer(t *testing.T) {
	t.Parallel()
	_, err := NewEventConsumerBuilder("e").WithEventType("external").Build()
	if err == nil {
		t.Fatal("want error")
	}
}

func TestConsumerBuilder_invalidPath(t *testing.T) {
	t.Parallel()
	_, err := NewConsumerBuilder().
		WithServiceName("s").
		WithConsumerType("persistent").
		WithHost("http://h").
		WithPath("no-leading-slash").
		Build()
	if err == nil {
		t.Fatal("want error")
	}
}
