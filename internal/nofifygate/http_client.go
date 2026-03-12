package nofifygate

import (
	"context"
	"encoding/json"
	"time"

	"github.com/IsaacDSC/clienthttp"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
)

type HttpClientImpl interface {
	orchestrator.Queue
}

var _ HttpClientImpl = &HttpClient{}

type HttpClient struct {
}

func NewHttpClient() *HttpClient {
	return &HttpClient{}
}

func (h HttpClient) Send(ctx context.Context, u string, payload orchestrator.Input, conf rule.Configs) error {
	var clientOpts []clienthttp.Option

	if conf.MaxTimeout != "" {
		if d, err := time.ParseDuration(conf.MaxTimeout); err == nil {
			clientOpts = append(clientOpts, clienthttp.WithTimeout(d))
		}
	}

	if conf.MaxRetry > 0 {
		strategy := clienthttp.NewExponentialBackoff()
		strategy.MaxAttempts = conf.MaxRetry
		clientOpts = append(clientOpts, clienthttp.WithRetryStrategy(strategy))
	}

	var (
		client *clienthttp.Client
		err    error
	)

	client, err = clienthttp.New(u, clientOpts...)
	if err != nil {
		return err
	}

	headers := map[string]string{
		"Content-Type":     "application/json",
		"x-transaction-id": payload.OrchestratorID.String(),
	}

	for k, values := range payload.Headers {
		if len(values) == 0 {
			continue
		}
		headers[k] = values[0]
	}

	var body []byte
	if payload.Data != nil {
		body, err = json.Marshal(payload.Data)
		if err != nil {
			return err
		}
	}

	_, err = client.Post(ctx, "", body, clienthttp.WithHeaders(headers))
	return err
}
