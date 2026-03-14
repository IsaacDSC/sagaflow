package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/IsaacDSC/sagaflow/internal/rule"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/mocks/mockstore"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestPsqlImpl_Save_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	r := rule.Rule{
		ID:   id,
		Name: "psql-rule",
		Transactions: []rule.HTTPConfig{{Method: "POST", URL: "http://svc/do"}},
		Rollback:     []rule.HTTPConfig{{Method: "POST", URL: "http://svc/undo"}},
	}

	mock := mockstore.NewMockPsqlImpl(ctrl)
	mock.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rule rule.Rule) (uuid.UUID, error) {
			if rule.Name != "psql-rule" {
				t.Errorf("Save rule.Name = %q, want psql-rule", rule.Name)
			}
			return id, nil
		}).
		Times(1)

	var impl store.PsqlImpl = mock
	gotID, err := impl.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if gotID != id {
		t.Errorf("Save() id = %v, want %v", gotID, id)
	}
}

func TestPsqlImpl_Save_Error_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	saveErr := errors.New("db error")
	r := rule.Rule{Name: "fail-rule", Transactions: nil, Rollback: nil}

	mock := mockstore.NewMockPsqlImpl(ctrl)
	mock.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(uuid.Nil, saveErr).Times(1)

	var impl store.PsqlImpl = mock
	_, err := impl.Save(ctx, r)
	if !errors.Is(err, saveErr) {
		t.Errorf("Save() err = %v, want %v", err, saveErr)
	}
}

func TestPsqlImpl_FindAll_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	id := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	rules := []rule.Rule{
		{
			ID:   id,
			Name: "rule-one",
			Transactions: []rule.HTTPConfig{{Method: "GET", URL: "http://example.com"}},
			Rollback:     nil,
		},
	}

	mock := mockstore.NewMockPsqlImpl(ctrl)
	mock.EXPECT().
		FindAll(gomock.Any()).
		Return(rules, nil).Times(1)

	var impl store.PsqlImpl = mock
	got, err := impl.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(got) != 1 || got[0].ID != id || got[0].Name != "rule-one" {
		t.Errorf("FindAll() = %+v, want %+v", got, rules)
	}
}

func TestPsqlImpl_FindAll_Error_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.Background()
	findErr := errors.New("query failed")

	mock := mockstore.NewMockPsqlImpl(ctrl)
	mock.EXPECT().
		FindAll(gomock.Any()).
		Return(nil, findErr).Times(1)

	var impl store.PsqlImpl = mock
	_, err := impl.FindAll(ctx)
	if !errors.Is(err, findErr) {
		t.Errorf("FindAll() err = %v, want %v", err, findErr)
	}
}
