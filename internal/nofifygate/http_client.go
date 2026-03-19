package nofifygate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/IsaacDSC/clienthttp"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
)

type HttpClientImpl interface {
	orchestrator.Publisher
}

var _ HttpClientImpl = &HttpClient{}

type HttpClient struct {
}

func NewHttpClient() *HttpClient {
	return &HttpClient{}
}

func (h HttpClient) Send(ctx context.Context, httpCfg rule.HTTPConfig, payload orchestrator.Input, conf rule.Configs) error {
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

	baseURL, path, err := splitBaseURLAndPath(httpCfg.URL)
	if err != nil {
		return fmt.Errorf("failed to split base URL and path: %w", err)
	}

	client, err = clienthttp.New(baseURL, clientOpts...)
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

	// path evita barra final que faria a lib montar baseURL + "/" e causar redirect 301/302 → GET
	_, err = client.Do(ctx, httpCfg.Method, path, body, clienthttp.WithHeaders(headers))
	return err
}

// splitBaseURLAndPath returns (scheme+host, path) to avoid that clienthttp adds "/" at the end
// and causes redirect 301/302 → GET
func splitBaseURLAndPath(rawURL string) (baseURL, path string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", err
	}
	baseURL = u.Scheme + "://" + u.Host
	path = strings.TrimPrefix(u.Path, "/")
	return baseURL, path, nil
}
