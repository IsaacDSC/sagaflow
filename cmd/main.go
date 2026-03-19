package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/entry"
	"github.com/IsaacDSC/sagaflow/internal/health"
	"github.com/IsaacDSC/sagaflow/internal/nofifygate"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/putrule"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()
	ctx = logger.WithLogger(ctx, logger.DefaultLogger)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		panic(err)
	}

	psqlStore := store.NewPsql(db)
	memStore := store.NewMemory()

	if err := loadMemRules(ctx, psqlStore, memStore); err != nil {
		panic(err)
	}

	gate := nofifygate.NewHttpClient()
	transactionParallel := orchestrator.NewTransactionParallel(memStore, gate)
	transactionNonParallel := orchestrator.NewTransactionNonParallel(memStore, gate)
	rollbackParallel := orchestrator.NewRollbackParallel(psqlStore, gate)
	orchestratorService := orchestrator.New(memStore, psqlStore, transactionParallel, transactionNonParallel, rollbackParallel)

	handlers := []connector.Handler{
		health.Handler(),
		entry.Handler(orchestratorService),
		putrule.Handler(psqlStore),
	}

	mux := http.NewServeMux()
	for _, ch := range handlers {
		mux.HandleFunc(ch.Path, connector.Adapter(ch.Handler))
	}

	go refreshRules(ctx, psqlStore, memStore)

	logger.Info(ctx, "server is running", "port", 3001)
	srv := &http.Server{
		Addr:         ":3001",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 2 * time.Minute,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	log.Fatal(srv.ListenAndServe())
}

func refreshRules(ctx context.Context, psqlStore store.PsqlImpl, memStore store.MemoryImpl) {
	ticker := time.NewTicker(1 * time.Minute)
	logger.Info(ctx, "starting rules refresh", "interval", time.Minute)
	for {
		select {
		case <-ticker.C:
			logger.Debug(ctx, "refreshing rules", "interval", time.Minute)
			if err := loadMemRules(ctx, psqlStore, memStore); err != nil {
				logger.Error(ctx, "failed to load memory rules", "error", err)
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}

func loadMemRules(ctx context.Context, psqlStore store.PsqlImpl, memStore store.MemoryImpl) error {
	rules, err := psqlStore.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to find all rules: %w", err)
	}

	if err := memStore.Refresh(ctx, rules); err != nil {
		return fmt.Errorf("failed to refresh memory store: %w", err)
	}

	return nil
}

func rollbackTransactions(ctx context.Context, psqlStore store.PsqlImpl, orchestratorService orchestrator.Orchestrator) {
	ticker := time.NewTicker(1 * time.Minute)
	logger.Info(ctx, "starting rollback transactions", "interval", time.Minute)
	for {
		select {
		case <-ticker.C:
			transactions, err := psqlStore.GetTransactions(ctx, store.StatusFailedExecuteRollback)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			if err != nil {
				logger.Error(ctx, "failed to get transactions", "error", err)
				continue
			}

			for _, transaction := range transactions {
				if err := orchestratorService.Rollback(ctx, transaction); err != nil {
					logger.Error(ctx, "failed to rollback transaction", "error", err)
					continue
				}
			}

			logger.Debug(ctx, "rolling back transactions", "interval", time.Minute)

		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
