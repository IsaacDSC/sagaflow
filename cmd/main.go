package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/IsaacDSC/sagaflow/internal/entry"
	"github.com/IsaacDSC/sagaflow/internal/nofifygate"
	"github.com/IsaacDSC/sagaflow/internal/orchestrator"
	"github.com/IsaacDSC/sagaflow/internal/putrule"
	"github.com/IsaacDSC/sagaflow/internal/store"
	"github.com/IsaacDSC/sagaflow/pkg/connector"
	_ "github.com/lib/pq"
)

func main() {
	ctx := context.Background()

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
	orchestratorService := orchestrator.New(memStore, gate)

	handlers := []connector.Handler{
		entry.Handler(orchestratorService),
		putrule.Handler(psqlStore),
	}

	mux := http.NewServeMux()
	for _, ch := range handlers {
		mux.HandleFunc(ch.Path, connector.Adapter(ch.Handler))
	}

	go refreshRules(ctx, psqlStore, memStore)

	log.Println("server is running on port 3001")
	log.Fatal(http.ListenAndServe(":3001", mux))
}

func refreshRules(ctx context.Context, psqlStore store.PsqlImpl, memStore store.MemoryImpl) {
	ticker := time.NewTicker(1 * time.Minute)
	fmt.Println("[*] Refreshing rules every 1 minute")
	for {
		select {
		case <-ticker.C:
			if err := loadMemRules(ctx, psqlStore, memStore); err != nil {
				log.Println("failed to load memory rules", err)
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
