package gqueue

// UpsertEventConsumerInput is the JSON body for PUT /api/v1/event/consumer.
type UpsertEventConsumerInput struct {
	Name      string           `json:"name"`
	Type      string           `json:"type"`
	Option    EventOption      `json:"option"`
	Consumers []Consumer       `json:"consumers"`
}

// EventOption configures queue behavior for an event.
type EventOption struct {
	WQType     string `json:"wq_type"`
	MaxRetries int    `json:"max_retries"`
	Retention  string `json:"retention"`
}

// Consumer describes a webhook consumer for an event.
type Consumer struct {
	ServiceName string            `json:"service_name"`
	Type        string            `json:"type"`
	Host        string            `json:"host"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers,omitempty"`
}
