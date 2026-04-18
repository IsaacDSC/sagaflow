package gqueue

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

// ClientOption configures NewClient.
type ClientOption func(*clientConfig) error

type clientConfig struct {
	baseURL      string
	httpClient   *http.Client
	basicUser    string
	basicPass    string
	requestHook  func(*http.Request) error
	userAgent    string
}

// NewClient builds a Client with functional options. WithBaseURL is required (directly or implicitly).
func NewClient(opts ...ClientOption) (*Client, error) {
	cfg := &clientConfig{
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(cfg.baseURL) == "" {
		return nil, fmt.Errorf("gqueue: base URL is required (use WithBaseURL)")
	}
	u, err := url.Parse(cfg.baseURL)
	if err != nil {
		return nil, fmt.Errorf("gqueue: parse base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("gqueue: base URL scheme must be http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("gqueue: base URL must include host")
	}
	base := strings.TrimRight(cfg.baseURL, "/")

	var hc *http.Client
	if cfg.httpClient != nil {
		hc = cfg.httpClient
	} else {
		hc = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return &Client{
		baseURL:     base,
		httpClient:  hc,
		basicUser:   cfg.basicUser,
		basicPass:   cfg.basicPass,
		requestHook: cfg.requestHook,
		userAgent:   cfg.userAgent,
	}, nil
}

// WithBaseURL sets the server root (e.g. http://localhost:8080). Trailing slashes are trimmed.
func WithBaseURL(raw string) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.baseURL = strings.TrimSpace(raw)
		return nil
	}
}

// WithHTTPClient sets the HTTP client used for requests. When nil, the option is ignored.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(cfg *clientConfig) error {
		if hc != nil {
			cfg.httpClient = hc
		}
		return nil
	}
}

// WithBasicAuth sets Authorization: Basic on every request.
func WithBasicAuth(username, password string) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.basicUser = username
		cfg.basicPass = password
		return nil
	}
}

// WithRequestHook runs on each request after default headers and auth are applied.
func WithRequestHook(hook func(*http.Request) error) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.requestHook = hook
		return nil
	}
}

// WithUserAgent sets the User-Agent header when non-empty.
func WithUserAgent(ua string) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.userAgent = strings.TrimSpace(ua)
		return nil
	}
}
