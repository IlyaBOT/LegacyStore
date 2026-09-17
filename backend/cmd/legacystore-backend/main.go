package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/api"
	"legacystore/backend/internal/catalog"
	"legacystore/backend/internal/catalogsign"
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

	accountStore := account.NewStore(conn)
	if strings.TrimSpace(cfg.AdminEmail) != "" || strings.TrimSpace(cfg.AdminPassword) != "" {
		if strings.TrimSpace(cfg.AdminEmail) == "" || strings.TrimSpace(cfg.AdminPassword) == "" {
			log.Fatal("admin provisioning requires both ADMIN_EMAIL and ADMIN_PASSWORD")
		}
		if err := accountStore.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword, cfg.AdminNickname); err != nil {
			log.Fatalf("admin provisioning failed: %v", err)
		}
		log.Printf("production administrator ensured for %s", cfg.AdminEmail)
	}
	hasAdmin, err := accountStore.HasAdmin(ctx)
	if err != nil {
		log.Fatalf("admin state check failed: %v", err)
	}
	if !hasAdmin {
		log.Fatal("no administrator is provisioned; set ADMIN_EMAIL and ADMIN_PASSWORD before starting the backend")
	}

	var signer *catalogsign.Signer
	if cfg.CatalogSigningEnabled {
		signer, err = catalogsign.Load(cfg.CatalogPrivateKeyPath, cfg.CatalogPublicKeyPath)
		if err != nil {
			log.Fatalf("catalog signing configuration failed: %v", err)
		}
		log.Printf("catalog signing enabled with key %s", signer.KeyID())
	}

	handler := api.NewRouterWithSigner(cfg, catalog.NewStore(conn), accountStore, signer)
	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.SecurityMiddleware(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
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
