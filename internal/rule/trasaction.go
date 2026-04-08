package rule

import (
	"errors"
	"net/url"

	"github.com/google/uuid"
)

type (
	Rule struct {
		ID           uuid.UUID    `json:"id" yaml:"id"`
		Name         string       `json:"name" yaml:"name"`
		Transactions []HTTPConfig `json:"transactions" yaml:"transactions"`
		Rollback     []HTTPConfig `json:"rollback" yaml:"rollback"`
		Configs      Configs      `json:"configs" yaml:"configs"`
	}

	HTTPConfig struct {
		ServiceName string `json:"service_name" yaml:"service_name"`
		Method      string `json:"method" yaml:"method"`
		URL         string `json:"url" yaml:"url"`
	}

	Configs struct {
		MaxRetry   int    `json:"max_retry" yaml:"max_retry"`
		MaxTimeout string `json:"max_timeout" yaml:"max_timeout"` // "1m"
		Parallel   bool   `json:"parallel" yaml:"parallel"`
		Sync       bool   `json:"sync" yaml:"sync"`
	}

	Status string
)

func (r Rule) Validate() error {
	if len(r.Transactions) == 0 {
		return errors.New("transactions is empty")
	}

	for _, transaction := range r.Transactions {
		if err := transaction.Validate(); err != nil {
			return err
		}
	}

	for _, rollback := range r.Rollback {
		if err := rollback.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (h HTTPConfig) Validate() error {
	if h.ServiceName == "" {
		return errors.New("service name is required")
	}

	if h.Method == "" {
		return errors.New("method is required")
	}

	if _, err := url.Parse(h.URL); err != nil {
		return errors.New("invalid url")
	}

	return nil
}

func (s Status) String() string {
	return string(s)
}

const (
	StatusRollbackExecuted      Status = "rollback_executed"
	StatusFailedExecuteRollback Status = "failed_execute_rollback"
)
