package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
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
		ID:           id,
		Name:         "psql-rule",
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
			ID:           id,
			Name:         "rule-one",
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

func TestPsql_SaveTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	orchID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440010")
	txID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440011")
	input := orchestrator.Transaction{
		TransactionID:  txID,
		OrchestratorID: orchID,
		Data:           map[string]string{"key": "value"},
		Headers:        map[string][]string{"X-Request-Id": {"req-123"}},
		ConfigRules:    nil,
		Error:          nil,
	}
	errorMsg := "rollback failed"

	dataJSON, _ := json.Marshal(input.Data)
	headersJSON, _ := json.Marshal(input.Headers)
	configRulesJSON, _ := json.Marshal(input.ConfigRules)

	mock.ExpectExec("INSERT INTO transactions").
		WithArgs(orchID, txID, dataJSON, headersJSON, store.StatusFailedExecuteRollback, errorMsg, configRulesJSON).
		WillReturnResult(sqlmock.NewResult(0, 1))

	p := store.NewPsql(db)
	err = p.SaveTransaction(ctx, input, errorMsg)
	if err != nil {
		t.Fatalf("SaveTransaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPsql_SaveTransaction_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	input := orchestrator.Transaction{
		TransactionID:  uuid.New(),
		OrchestratorID: uuid.New(),
		Data:           nil,
		Headers:        nil,
		ConfigRules:    nil,
		Error:          nil,
	}
	dbErr := errors.New("insert failed")

	mock.ExpectExec("INSERT INTO transactions").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(dbErr)

	p := store.NewPsql(db)
	err = p.SaveTransaction(ctx, input, "err")
	if err == nil {
		t.Fatal("SaveTransaction: expected error")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("SaveTransaction() err = %v, want %v", err, dbErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPsql_UpdateTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	txID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440020")

	mock.ExpectExec("UPDATE transactions").
		WithArgs(store.StatusRollbackExecuted, txID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	p := store.NewPsql(db)
	err = p.UpdateTransaction(ctx, txID)
	if err != nil {
		t.Fatalf("UpdateTransaction: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPsql_UpdateTransaction_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	txID := uuid.New()
	dbErr := errors.New("update failed")

	mock.ExpectExec("UPDATE transactions").
		WithArgs(store.StatusRollbackExecuted, txID).
		WillReturnError(dbErr)

	p := store.NewPsql(db)
	err = p.UpdateTransaction(ctx, txID)
	if err == nil {
		t.Fatal("UpdateTransaction: expected error")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("UpdateTransaction() err = %v, want %v", err, dbErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPsql_GetTransactions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	txID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440030")
	dataJSON := []byte(`{"key":"value"}`)
	headersJSON := []byte(`{"X-Request-Id":["req-1"]}`)
	configRulesJSON := []byte(`[]`)
	status := store.StatusFailedExecuteRollback

	rows := sqlmock.NewRows([]string{"transaction_id", "data", "headers", "config_rules"}).
		AddRow(txID, dataJSON, headersJSON, configRulesJSON)

	mock.ExpectQuery("SELECT transaction_id, data, headers, config_rules").
		WithArgs(status).
		WillReturnRows(rows)

	p := store.NewPsql(db)
	got, err := p.GetTransactions(ctx, status)
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GetTransactions() len = %d, want 1", len(got))
	}
	if got[0].TransactionID != txID {
		t.Errorf("GetTransactions()[0].TransactionID = %v, want %v", got[0].TransactionID, txID)
	}
	if got[0].Data == nil {
		t.Error("GetTransactions()[0].Data = nil, want unmarshaled data")
	}
	if got[0].Headers == nil {
		t.Error("GetTransactions()[0].Headers = nil, want unmarshaled headers")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPsql_GetTransactions_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	status := store.StatusFailedExecuteRollback

	rows := sqlmock.NewRows([]string{"transaction_id", "data", "headers", "config_rules"})
	mock.ExpectQuery("SELECT transaction_id, data, headers, config_rules").
		WithArgs(status).
		WillReturnRows(rows)

	p := store.NewPsql(db)
	got, err := p.GetTransactions(ctx, status)
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("GetTransactions() len = %d, want 0", len(got))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestPsql_GetTransactions_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	dbErr := errors.New("query failed")
	mock.ExpectQuery("SELECT transaction_id, data, headers, config_rules").
		WithArgs(store.StatusFailedExecuteRollback).
		WillReturnError(dbErr)

	p := store.NewPsql(db)
	_, err = p.GetTransactions(ctx, store.StatusFailedExecuteRollback)
	if err == nil {
		t.Fatal("GetTransactions: expected error")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("GetTransactions() err = %v, want %v", err, dbErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
