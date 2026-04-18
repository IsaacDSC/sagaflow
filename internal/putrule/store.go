package putrule

//go:generate mockgen -source=store.go -destination=../../mocks/mockputrule/mock_http_handle.go -package=mockputrule

import (
	"context"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/google/uuid"
)

// Store persists rules (same contract as orchestrator.RuleStore; used by mocks).
type Store interface {
	Save(ctx context.Context, rule rule.Rule) (uuid.UUID, error)
}
