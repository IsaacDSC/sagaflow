package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/IsaacDSC/sagaflow/cmd/internal/task"
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

	if err := task.LoadMemRules(ctx, psqlStore, memStore); err != nil {
		panic(err)
	}

	gate := nofifygate.NewHttpClient()
	transactionParallel := orchestrator.NewTransactionParallel(memStore, gate)
	transactionNonParallel := orchestrator.NewTransactionNonParallel(memStore, gate)
	rollbackParallel := orchestrator.NewRollbackParallel(psqlStore, gate)
	orchestratorService := orchestrator.New(
		memStore,
		psqlStore,
		transactionParallel,
		transactionNonParallel,
		rollbackParallel,
	)

	handlers := []connector.Handler{
		health.Handler(),
		entry.Handler(orchestratorService),
		putrule.Handler(psqlStore),
	}

	mux := http.NewServeMux()
	for _, ch := range handlers {
		mux.HandleFunc(ch.Path, connector.Adapter(ch.Handler))
	}

	go task.RefreshRules(ctx, psqlStore, memStore)
	go task.FailRollbacks(ctx, orchestratorService)

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
