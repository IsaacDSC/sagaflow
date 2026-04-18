package gqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	pathEventConsumer = "/api/v1/event/consumer"
	pathPubsub        = "/api/v1/pubsub"
	maxResponseBytes  = 10 << 20 // 10 MiB
)

// Client is an HTTP client for the gqueue API.
type Client struct {
	baseURL     string
	httpClient  *http.Client
	basicUser   string
	basicPass   string
	requestHook func(*http.Request) error
	userAgent   string
}

// UpsertEventConsumer registers or updates an event and its consumers (PUT /api/v1/event/consumer).
func (c *Client) UpsertEventConsumer(ctx context.Context, input UpsertEventConsumerInput) error {
	if c == nil {
		return fmt.Errorf("gqueue: nil Client")
	}
	return c.doJSON(ctx, http.MethodPut, pathEventConsumer, &input, nil)
}

// Publish sends a pub/sub message (POST /api/v1/pubsub).
func (c *Client) Publish(ctx context.Context, input PublishInput) error {
	if c == nil {
		return fmt.Errorf("gqueue: nil Client")
	}
	return c.doJSON(ctx, http.MethodPost, pathPubsub, &input, nil)
}

// doJSON sends a JSON request. respOut, when non-nil, is unmarshaled only if the status is 2xx and the body is non-empty.
// Decoding allows unknown fields for forward compatibility (see spec).
func (c *Client) doJSON(ctx context.Context, method, path string, reqBody, respOut any) error {
	var bodyReader io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("gqueue: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return fmt.Errorf("gqueue: new request: %w", err)
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if c.basicUser != "" || c.basicPass != "" {
		req.SetBasicAuth(c.basicUser, c.basicPass)
	}
	if c.requestHook != nil {
		if err := c.requestHook(req); err != nil {
			return fmt.Errorf("gqueue: request hook: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gqueue: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("gqueue: read response: %w", err)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &APIError{StatusCode: resp.StatusCode, Body: respBytes}
	}

	if respOut != nil && len(bytes.TrimSpace(respBytes)) > 0 {
		if err := json.Unmarshal(respBytes, respOut); err != nil {
			return fmt.Errorf("gqueue: decode response: %w", err)
		}
	}
	return nil
}
