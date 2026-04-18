package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IsaacDSC/sagaflow/cmd/internal/task"
	"github.com/IsaacDSC/sagaflow/internal/cfg"
	"github.com/IsaacDSC/sagaflow/internal/entry"
	"github.com/IsaacDSC/sagaflow/internal/health"
	"github.com/IsaacDSC/sagaflow/internal/nofifygate"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/putrule"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	"github.com/IsaacDSC/sagaflow/pkg/gqueue"
	"github.com/IsaacDSC/sagaflow/pkg/logger"
	_ "github.com/lib/pq"
)

func main() {
	baseCtx := context.Background()
	baseCtx = logger.WithLogger(baseCtx, logger.DefaultLogger)
	bgCtx, signalStop := signal.NotifyContext(baseCtx, syscall.SIGINT, syscall.SIGTERM)
	defer signalStop()

	if err := cfg.Load(); err != nil {
		panic(err)
	}

	conf := cfg.Get()
	db, err := sql.Open("postgres", conf.Database.URL)
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

	if err := task.LoadMemRules(bgCtx, psqlStore, memStore); err != nil {
		panic(err)
	}

	gate := nofifygate.NewHttpClient()
	transactionParallel := orchestrator.NewTransactionParallel(memStore, gate)
	transactionNonParallel := orchestrator.NewTransactionNonParallel(memStore, gate)
	rollbackParallel := orchestrator.NewRollbackParallel(psqlStore, gate)

	var (
		gqClient           gqueue.API
		transactionAsync   orchestrator.TransactionAsyncUseCase
	)
	if conf.Gqueue.BaseURL != "" {
		c, err := gqueue.NewClient(
			gqueue.WithBaseURL(conf.Gqueue.BaseURL),
			gqueue.WithBasicAuth(conf.Gqueue.BasicUser, conf.Gqueue.BasicPassword),
		)
		if err != nil {
			panic(err)
		}
		gqClient = c
		transactionAsync = orchestrator.NewTransactionAsync(gqClient, conf.Gqueue.PublisherServiceName)
	}

	orchestratorService := orchestrator.New(
		memStore,
		psqlStore,
		transactionParallel,
		transactionNonParallel,
		transactionAsync,
		rollbackParallel,
		gqClient,
		psqlStore,
	)

	handlers := []connector.Handler{
		health.Handler(),
		entry.Handler(orchestratorService),
		putrule.Handler(orchestratorService),
	}

	mux := http.NewServeMux()
	for _, ch := range handlers {
		mux.HandleFunc(ch.Path, connector.Adapter(ch.Handler))
	}

	go task.RefreshRules(bgCtx, psqlStore, memStore)
	go task.FailRollbacks(bgCtx, orchestratorService)

	logger.Info(baseCtx, "server is running", "port", 3001)
	srv := &http.Server{
		Addr:         ":3001",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 2 * time.Minute,
		BaseContext: func(l net.Listener) context.Context {
			// Do not use bgCtx here: canceling the parent context cancels request.Context() and can
			// interrupt in-flight work before srv.Shutdown gets a chance to wait.
			return baseCtx
		},
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(baseCtx, "server listen error", "error", err)
			signalStop()
			os.Exit(1)
		}
		return
	case <-bgCtx.Done():
		logger.Info(baseCtx, "shutdown requested")
	}

	signalStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error(baseCtx, "server shutdown failed", "error", err)
	}

	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(baseCtx, "server stopped with error", "error", err)
		os.Exit(1)
	}
}
