package rule

import (
	"github.com/google/uuid"
)

type (
	Rule struct {
		ID           uuid.UUID           `json:"id" yaml:"id"`
		Name         string              `json:"name" yaml:"name"`
		Transactions []HTTPConfig        `json:"transactions" yaml:"transactions"`
		Rollback     []HTTPConfig        `json:"rollback" yaml:"rollback"`
		Transforms   []map[string]string `json:"transforms" yaml:"transforms"` // ex: []{ {"tx1": "http://..."}, {"tx2": "..."} }
		Configs      Configs             `json:"configs" yaml:"configs"`
	}

	HTTPConfig struct {
		Method string `json:"method" yaml:"method"`
		URL    string `json:"url" yaml:"url"`
	}

	Configs struct {
		MaxRetry   int    `json:"max_retry" yaml:"max_retry"`
		MaxTimeout string `json:"max_timeout" yaml:"max_timeout"` // "1m"
		Parallel   bool   `json:"parallel" yaml:"parallel"`
		Sync       bool   `json:"sync" yaml:"sync"`
	}

	Status string
)

func (s Status) String() string {
	return string(s)
}

const (
	StatusRollbackExecuted      Status = "rollback_executed"
	StatusFailedExecuteRollback Status = "failed_execute_rollback"
)
