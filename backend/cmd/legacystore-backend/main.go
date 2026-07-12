package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/api"
	"legacystore/backend/internal/catalog"
	"legacystore/backend/internal/config"
	"legacystore/backend/internal/db"
)

func main() {
	cfg := config.Load()

	ctx := context.Background()
	conn, err := db.Open(ctx, cfg)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer conn.Close()

	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.NewRouter(cfg, catalog.NewStore(conn), account.NewStore(conn)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("LegacyStore backend listening on %s", cfg.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("backend failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("backend shutdown failed: %v", err)
	}
}
