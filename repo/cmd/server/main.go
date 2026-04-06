package main

// @title ParkOps API
// @version 1.0
// @description ParkOps Command & Reservation Platform - offline-first parking operations
// @host localhost:8080
// @BasePath /api
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name session_id

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"parkops/internal/auth"
	"parkops/internal/campaigns"
	"parkops/internal/config"
	"parkops/internal/db"
	"parkops/internal/exports"
	"parkops/internal/notifications"
	"parkops/internal/reconciliation"
	"parkops/internal/segments"
	"parkops/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := server.NewLogger(cfg.AppEnv)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DatabaseURL, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	if err := db.BackfillSigningSecrets(ctx, pool, cfg.EncryptionKey, logger); err != nil {
		logger.Error("failed to backfill signing secrets", "error", err)
		os.Exit(1)
	}

	exportStore, err := exports.NewFileStore(cfg.ExportStorageDir)
	if err != nil {
		logger.Error("failed to create export storage", "error", err)
		os.Exit(1)
	}

	r := server.NewRouter(logger, pool, cfg.EncryptionKey, exportStore)
	reconcileService := reconciliation.NewService(pool, auth.NewService(auth.NewPostgresStore(pool)))
	go reconciliation.StartScheduler(ctx, logger, reconcileService)
	notificationService := notifications.NewService(pool)
	go notifications.StartProcessor(ctx, logger, notificationService)
	campaignService := campaigns.NewService(pool)
	go campaigns.StartReminderScheduler(ctx, logger, campaignService)
	segmentService := segments.NewService(pool)
	nightlyCfg := segments.NightlyConfig{
		Hour:     cfg.NightlySchedule.Hour,
		Minute:   cfg.NightlySchedule.Minute,
		Timezone: cfg.NightlySchedule.Timezone,
	}
	go segments.StartNightlyScheduler(ctx, logger, segmentService, nightlyCfg)

	httpServer := &http.Server{
		Addr:              cfg.AppAddr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("starting http server", "addr", cfg.AppAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
