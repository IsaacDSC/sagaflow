package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/IsaacDSC/sagaflow/internal/putrule"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/google/uuid"
)

type PsqlImpl interface {
	putrule.Store
	FindAll(ctx context.Context) ([]rule.Rule, error)
}

var _ PsqlImpl = &Psql{}

type Psql struct {
	db *sql.DB
}

func NewPsql(db *sql.DB) *Psql {
	return &Psql{db: db}
}

type (
	Rule struct {
		ID           uuid.UUID `json:"id" yaml:"id"`
		Name         string    `json:"name" yaml:"name"`
		Transactions []byte    `json:"transactions" yaml:"transactions"`
		Rollback     []byte    `json:"rollback" yaml:"rollback"`
		Transforms   []byte    `json:"transforms" yaml:"transforms"` // ex: []{ {"tx1": "http://..."}, {"tx2": "..."} }
		Configs      []byte    `json:"configs" yaml:"configs"`
	}

	Configs struct {
		MaxRetry   int    `json:"max_retry" yaml:"max_retry"`
		MaxTimeout string `json:"max_timeout" yaml:"max_timeout"` // "1m"
		Parallel   bool   `json:"parallel" yaml:"parallel"`
		Sync       bool   `json:"sync" yaml:"sync"`
	}
)

func (r Rule) rule() rule.Rule {
	var (
		tx         []rule.HTTPConfig
		rollback   []rule.HTTPConfig
		transforms []map[string]string
		configs    rule.Configs
	)

	_ = json.Unmarshal(r.Transactions, &tx)
	_ = json.Unmarshal(r.Rollback, &rollback)
	_ = json.Unmarshal(r.Transforms, &transforms)
	_ = json.Unmarshal(r.Configs, &configs)

	return rule.Rule{
		ID:           r.ID,
		Name:         r.Name,
		Transactions: tx,
		Rollback:     rollback,
		Transforms:   transforms,
		Configs:      configs,
	}
}

func ruleToModel(rule rule.Rule) Rule {
	tx, _ := json.Marshal(rule.Transactions)
	rollback, _ := json.Marshal(rule.Rollback)
	transform, _ := json.Marshal(rule.Transforms)
	configs, _ := json.Marshal(rule.Configs)

	return Rule{
		ID:           rule.ID,
		Name:         rule.Name,
		Transactions: tx,
		Rollback:     rollback,
		Transforms:   transform,
		Configs:      configs,
	}
}

func (b Psql) Save(ctx context.Context, rule rule.Rule) (uuid.UUID, error) {
	const query = `
		INSERT INTO rules (id, name, transactions, rollback, transforms, configs)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (name) DO UPDATE SET
		transactions = $3, rollback = $4, transforms = $5, configs = $6
		RETURNING id
	`

	model := ruleToModel(rule)

	row := b.db.QueryRowContext(ctx, query, model.ID, model.Name, model.Transactions, model.Rollback, model.Transforms, model.Configs)
	var id uuid.UUID
	err := row.Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to upsert rule: %w", err)
	}

	return id, nil
}

func (b Psql) FindAll(ctx context.Context) ([]rule.Rule, error) {
	const query = `
		SELECT id, name, transactions, rollback, transforms, configs
		FROM rules
	`

	rows, err := b.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to find all rules: %w", err)
	}
	defer rows.Close()

	var rules []rule.Rule
	for rows.Next() {
		var r Rule
		err := rows.Scan(&r.ID, &r.Name, &r.Transactions, &r.Rollback, &r.Transforms, &r.Configs)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}

		rules = append(rules, r.rule())
	}

	return rules, nil
}
