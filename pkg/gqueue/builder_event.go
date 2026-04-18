package gqueue

import (
	"fmt"
	"net/url"
	"strings"
)

// EventConsumerBuilder constructs UpsertEventConsumerInput.
type EventConsumerBuilder struct {
	eventName string
	eventType string
	option    EventOption
	consumers []Consumer
}

// ConsumerBuilder constructs a single Consumer.
type ConsumerBuilder struct {
	c Consumer
}

// NewEventConsumerBuilder starts a builder for PUT /api/v1/event/consumer.
func NewEventConsumerBuilder(eventName string) *EventConsumerBuilder {
	return &EventConsumerBuilder{eventName: eventName}
}

// WithEventType sets the event type (e.g. "external").
func (b *EventConsumerBuilder) WithEventType(t string) *EventConsumerBuilder {
	if b != nil {
		b.eventType = t
	}
	return b
}

// WithOption sets queue options for the event.
func (b *EventConsumerBuilder) WithOption(opt EventOption) *EventConsumerBuilder {
	if b != nil {
		b.option = opt
	}
	return b
}

// AddConsumer appends a built Consumer.
func (b *EventConsumerBuilder) AddConsumer(c Consumer) *EventConsumerBuilder {
	if b != nil {
		b.consumers = append(b.consumers, c)
	}
	return b
}

// AddConsumerFromBuilder appends a consumer from a sub-builder.
func (b *EventConsumerBuilder) AddConsumerFromBuilder(cb *ConsumerBuilder) (*EventConsumerBuilder, error) {
	if b == nil {
		return nil, fmt.Errorf("gqueue: nil EventConsumerBuilder")
	}
	if cb == nil {
		return b, fmt.Errorf("gqueue: nil ConsumerBuilder")
	}
	c, err := cb.Build()
	if err != nil {
		return b, err
	}
	b.consumers = append(b.consumers, c)
	return b, nil
}

// Build validates and returns UpsertEventConsumerInput.
func (b *EventConsumerBuilder) Build() (UpsertEventConsumerInput, error) {
	if b == nil {
		return UpsertEventConsumerInput{}, fmt.Errorf("gqueue: nil EventConsumerBuilder")
	}
	if strings.TrimSpace(b.eventName) == "" {
		return UpsertEventConsumerInput{}, fmt.Errorf("gqueue: event name is required")
	}
	if strings.TrimSpace(b.eventType) == "" {
		return UpsertEventConsumerInput{}, fmt.Errorf("gqueue: event type is required")
	}
	if len(b.consumers) == 0 {
		return UpsertEventConsumerInput{}, fmt.Errorf("gqueue: at least one consumer is required")
	}
	for i := range b.consumers {
		if err := validateConsumer(&b.consumers[i]); err != nil {
			return UpsertEventConsumerInput{}, fmt.Errorf("gqueue: consumer %d: %w", i, err)
		}
	}
	return UpsertEventConsumerInput{
		Name:      b.eventName,
		Type:      b.eventType,
		Option:    b.option,
		Consumers: append([]Consumer(nil), b.consumers...),
	}, nil
}

// NewConsumerBuilder creates a sub-builder for one consumer.
func NewConsumerBuilder() *ConsumerBuilder {
	return &ConsumerBuilder{c: Consumer{Headers: make(map[string]string)}}
}

// WithServiceName sets service_name.
func (b *ConsumerBuilder) WithServiceName(name string) *ConsumerBuilder {
	if b != nil {
		b.c.ServiceName = name
	}
	return b
}

// WithConsumerType sets the consumer type (e.g. "persistent").
func (b *ConsumerBuilder) WithConsumerType(t string) *ConsumerBuilder {
	if b != nil {
		b.c.Type = t
	}
	return b
}

// WithHost sets the consumer base URL (e.g. http://consumer:3333).
func (b *ConsumerBuilder) WithHost(host string) *ConsumerBuilder {
	if b != nil {
		b.c.Host = host
	}
	return b
}

// WithPath sets the HTTP path (e.g. /payment/charged).
func (b *ConsumerBuilder) WithPath(path string) *ConsumerBuilder {
	if b != nil {
		b.c.Path = path
	}
	return b
}

// WithHeader adds a single header to the consumer request template.
func (b *ConsumerBuilder) WithHeader(key, value string) *ConsumerBuilder {
	if b == nil {
		return nil
	}
	if b.c.Headers == nil {
		b.c.Headers = make(map[string]string)
	}
	b.c.Headers[key] = value
	return b
}

// Build validates and returns Consumer.
func (b *ConsumerBuilder) Build() (Consumer, error) {
	if b == nil {
		return Consumer{}, fmt.Errorf("gqueue: nil ConsumerBuilder")
	}
	c := b.c
	if c.Headers == nil {
		c.Headers = make(map[string]string)
	}
	if err := validateConsumer(&c); err != nil {
		return Consumer{}, err
	}
	return c, nil
}

func validateConsumer(c *Consumer) error {
	if strings.TrimSpace(c.ServiceName) == "" {
		return fmt.Errorf("service_name is required")
	}
	if strings.TrimSpace(c.Type) == "" {
		return fmt.Errorf("consumer type is required")
	}
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("host is required")
	}
	u, err := url.Parse(strings.TrimSpace(c.Host))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("host must be an absolute URL with scheme and host")
	}
	if !strings.HasPrefix(c.Path, "/") {
		return fmt.Errorf("path must start with /")
	}
	return nil
}
