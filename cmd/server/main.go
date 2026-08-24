package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"configcenter/internal/config"
	"configcenter/internal/event"
	"configcenter/internal/repository"
	transport "configcenter/internal/transport/http"
)

func main() {
	if err := run(); err != nil {
		slog.Error("configuration center stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)
	store, err := repository.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer store.Close()
	hub := event.NewHub(cfg.EventHistory)
	defer hub.Close()
	handler := transport.NewServer(store, hub, cfg.AdminToken, cfg.Heartbeat, cfg.MaxBodyBytes, logger)
	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           handler.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       60 * time.Second,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("configuration center started", slog.String("address", cfg.Address), slog.String("database", cfg.DatabasePath))
		errorsChannel <- server.ListenAndServe()
	}()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case received := <-signals:
		logger.Info("shutdown requested", slog.String("signal", received.String()))
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	hub.Close()
	if err := server.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}

func newLogger(level string) *slog.Logger {
	var selected slog.Level
	switch strings.ToLower(level) {
	case "debug":
		selected = slog.LevelDebug
	case "warn":
		selected = slog.LevelWarn
	case "error":
		selected = slog.LevelError
	default:
		selected = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: selected}))
}
