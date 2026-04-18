package gqueue

import (
	"encoding/json"
	"fmt"
)

// PublishBuilder constructs PublishInput.
type PublishBuilder struct {
	serviceName string
	eventName   string
	data        map[string]any
	metadata    map[string]any
}

// NewPublishBuilder starts a builder for POST /api/v1/pubsub.
func NewPublishBuilder(serviceName, eventName string) *PublishBuilder {
	return &PublishBuilder{
		serviceName: serviceName,
		eventName:   eventName,
		data:        make(map[string]any),
		metadata:    make(map[string]any),
	}
}

// WithData replaces the entire data object.
func (b *PublishBuilder) WithData(data map[string]any) *PublishBuilder {
	if b == nil {
		return nil
	}
	if data == nil {
		b.data = make(map[string]any)
		return b
	}
	b.data = make(map[string]any, len(data))
	for k, v := range data {
		b.data[k] = v
	}
	return b
}

// WithDataFrom marshals v to JSON and unmarshals into a map for the data field.
func (b *PublishBuilder) WithDataFrom(v any) (*PublishBuilder, error) {
	if b == nil {
		return nil, fmt.Errorf("gqueue: nil PublishBuilder")
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return b, fmt.Errorf("gqueue: WithDataFrom marshal: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return b, fmt.Errorf("gqueue: WithDataFrom unmarshal to map: %w", err)
	}
	b.data = m
	return b, nil
}

// WithMetadata sets a single metadata key. CorrelationID can use WithCorrelationID instead.
func (b *PublishBuilder) WithMetadata(key, value string) *PublishBuilder {
	if b == nil {
		return nil
	}
	if b.metadata == nil {
		b.metadata = make(map[string]any)
	}
	b.metadata[key] = value
	return b
}

// WithCorrelationID sets metadata correlation_id.
func (b *PublishBuilder) WithCorrelationID(id string) *PublishBuilder {
	return b.WithMetadata("correlation_id", id)
}

// Build validates and returns PublishInput.
func (b *PublishBuilder) Build() (PublishInput, error) {
	if b == nil {
		return PublishInput{}, fmt.Errorf("gqueue: nil PublishBuilder")
	}
	if b.serviceName == "" {
		return PublishInput{}, fmt.Errorf("gqueue: service_name is required")
	}
	if b.eventName == "" {
		return PublishInput{}, fmt.Errorf("gqueue: event_name is required")
	}
	if b.data == nil {
		b.data = make(map[string]any)
	}
	in := PublishInput{
		ServiceName: b.serviceName,
		EventName:   b.eventName,
		Data:        b.data,
	}
	if len(b.metadata) > 0 {
		in.Metadata = b.metadata
	}
	return in, nil
}
