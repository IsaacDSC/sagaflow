package gqueue

import "context"

//go:generate mockgen -source=api.go -destination=../../mocks/mockgqueue/mock_api.go -package=mockgqueue

// API is the gqueue client surface; use with mockgen for tests.
type API interface {
	UpsertEventConsumer(ctx context.Context, input UpsertEventConsumerInput) error
	Publish(ctx context.Context, input PublishInput) error
}

var _ API = (*Client)(nil)
