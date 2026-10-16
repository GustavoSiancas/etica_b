package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"etica_back/internal/config"
	"etica_back/internal/httpapi"
	"etica_back/internal/platform/postgres"
	"etica_back/internal/platform/r2"
	"etica_back/internal/services"
)

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, pool, err := postgres.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("PostgreSQL: %s", postgres.Diagnose(err))
	}
	defer pool.Close()
	if applied, migrationErr := postgres.ApplyInitialMigration(ctx, db, "migrations/001_initial.up.sql"); migrationErr != nil {
		return fmt.Errorf("PostgreSQL initial migration: %s", postgres.Diagnose(migrationErr))
	} else if applied {
		log.Printf("Applied consolidated initial migration")
	}
	if err = r2.Check(ctx, r2.New(cfg), cfg.R2Bucket); err != nil {
		return fmt.Errorf("R2: %s", r2.Diagnose(err))
	}
	service, err := services.NewUploadService(db, r2.New(cfg), cfg.R2Bucket, cfg.R2PublicURL)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: httpapi.Routes(service, services.NewExpedienteService(db), services.NewDocumentService(db), services.NewParticipantService(db), httpapi.RoutesOption{Escrito: services.NewEscritoService(db), DemoData: services.NewDemoDataService(db, "seeds/demo_data.sql", cfg.EnableDemoReset)}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopSignal()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-stop.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server.Shutdown(shutdown)
		case <-done:
		}
	}()
	log.Printf("PostgreSQL and R2 connected. HTTP listening on %s", cfg.HTTPAddr)
	if err = server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
