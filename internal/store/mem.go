package store

import (
	"context"
	"sync/atomic"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/google/uuid"
)

type MemoryImpl interface {
	orchestrator.MemStore
	Refresh(ctx context.Context, rules []rule.Rule) error
}

var _ MemoryImpl = &Memory{}

type Memory struct {
	db atomic.Value // map[uuid.UUID]rule.Rule
}

func NewMemory() *Memory {
	return &Memory{}
}

func (m *Memory) Find(ctx context.Context, txID uuid.UUID) (rule.Rule, error) {
	rules, ok := m.db.Load().(map[uuid.UUID]rule.Rule)
	if !ok {
		return rule.Rule{}, orchestrator.ErrorRuleNotFound
	}

	rl, ok := rules[txID]
	if !ok {
		return rule.Rule{}, orchestrator.ErrorRuleNotFound
	}

	return rl, nil
}

func (m *Memory) Refresh(ctx context.Context, rules []rule.Rule) error {
	r := make(map[uuid.UUID]rule.Rule, len(rules))
	for _, rl := range rules {
		r[rl.ID] = rl
	}

	m.db.Store(r)

	return nil
}
