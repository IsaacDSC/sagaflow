package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/mocks/mockstore"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestMemory_Find_EmptyDB(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

	_, err := m.Find(ctx, id)
	if !errors.Is(err, orchestrator.ErrorRuleNotFound) {
		t.Errorf("Find() err = %v, want ErrorRuleNotFound", err)
	}
}

func TestMemory_Find_NotFound(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	idA := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	idB := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

	r := rule.Rule{
		ID:   idA,
		Name: "rule-a",
		Transactions: []rule.HTTPConfig{{Method: "POST", URL: "http://svc/do"}},
		Rollback:     []rule.HTTPConfig{{Method: "POST", URL: "http://svc/undo"}},
	}
	if err := m.Refresh(ctx, []rule.Rule{r}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	_, err := m.Find(ctx, idB)
	if !errors.Is(err, orchestrator.ErrorRuleNotFound) {
		t.Errorf("Find() err = %v, want ErrorRuleNotFound", err)
	}
}

func TestMemory_Find_Found(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	r := rule.Rule{
		ID:   id,
		Name: "my-rule",
		Transactions: []rule.HTTPConfig{{Method: "POST", URL: "http://svc/do"}},
		Rollback:     []rule.HTTPConfig{{Method: "POST", URL: "http://svc/undo"}},
		Configs:      rule.Configs{Parallel: true},
	}

	if err := m.Refresh(ctx, []rule.Rule{r}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	got, err := m.Find(ctx, id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ID != r.ID || got.Name != r.Name || !got.Configs.Parallel {
		t.Errorf("Find() = %+v, want rule %+v", got, r)
	}
}

func TestMemory_Refresh_Overwrites(t *testing.T) {
	m := store.NewMemory()
	ctx := context.Background()
	idA := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	idB := uuid.MustParse("550e8400-e29b-41d4-a716-446655440002")

	rA := rule.Rule{ID: idA, Name: "rule-a", Transactions: nil, Rollback: nil}
	rB := rule.Rule{ID: idB, Name: "rule-b", Transactions: nil, Rollback: nil}

	if err := m.Refresh(ctx, []rule.Rule{rA}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, err := m.Find(ctx, idA); err != nil {
		t.Fatalf("Find A: %v", err)
	}

	if err := m.Refresh(ctx, []rule.Rule{rB}); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if _, err := m.Find(ctx, idA); !errors.Is(err, orchestrator.ErrorRuleNotFound) {
		t.Errorf("Find(idA) after overwrite: err = %v, want ErrorRuleNotFound", err)
	}
	got, err := m.Find(ctx, idB)
	if err != nil {
		t.Fatalf("Find(idB): %v", err)
	}
	if got.Name != "rule-b" {
		t.Errorf("Find(idB).Name = %q, want %q", got.Name, "rule-b")
	}
}

// TestMemoryImpl_WithMock exercises the MemoryImpl interface using the mock.
func TestMemoryImpl_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	r := rule.Rule{
		ID:   id,
		Name: "mocked-rule",
		Transactions: []rule.HTTPConfig{{Method: "GET", URL: "http://example.com"}},
		Rollback:     nil,
	}

	mock := mockstore.NewMockMemoryImpl(ctrl)
	mock.EXPECT().
		Find(gomock.Any(), id).
		Return(r, nil).Times(1)

	var impl store.MemoryImpl = mock
	got, err := impl.Find(ctx, id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ID != id || got.Name != "mocked-rule" {
		t.Errorf("Find() = %+v, want rule with ID %v and Name mocked-rule", got, id)
	}
}
