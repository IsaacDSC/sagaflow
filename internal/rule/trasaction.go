package rule

import (
	"github.com/google/uuid"
)

type (
	Rule struct {
		ID           uuid.UUID           `json:"id" yaml:"id"`
		Name         string              `json:"name" yaml:"name"`
		Transactions []map[string]string `json:"transactions" yaml:"transactions"`
		Rollback     []map[string]string `json:"rollback" yaml:"rollback"`
		Transforms   []map[string]string `json:"transforms" yaml:"transforms"` // ex: []{ {"tx1": "http://..."}, {"tx2": "..."} }
		Configs      Configs             `json:"configs" yaml:"configs"`
	}

	Configs struct {
		MaxRetry   int    `json:"max_retry" yaml:"max_retry"`
		MaxTimeout string `json:"max_timeout" yaml:"max_timeout"` // "1m"
		Parallel   bool   `json:"parallel" yaml:"parallel"`
		Sync       bool   `json:"sync" yaml:"sync"`
	}
)
