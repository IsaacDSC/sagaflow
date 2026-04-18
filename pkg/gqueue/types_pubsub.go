package gqueue

// PublishInput is the JSON body for POST /api/v1/pubsub.
// Data uses map[string]any for ergonomic arbitrary payloads.
// Metadata is a JSON object (e.g. correlation_id); use builders for common fields.
type PublishInput struct {
	ServiceName string         `json:"service_name"`
	EventName   string         `json:"event_name"`
	Data        map[string]any `json:"data"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
