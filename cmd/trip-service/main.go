package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SizikovK/TripGo-MVP/internal/config"
	"github.com/SizikovK/TripGo-MVP/internal/httpserver"
	"github.com/SizikovK/TripGo-MVP/internal/repository"
	"github.com/SizikovK/TripGo-MVP/internal/trips"
	"github.com/jackc/pgx/v5/pgxpool"
)

func run() error {
	stopped := make(chan struct{})
	defer close(stopped)

	cfg, err := config.New()
	if err != nil {
		return fmt.Errorf("конфигурация: %w", err)
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	slog.Info("Запуск сервиса")

	dbPool, err := createPool(ctx, cfg)
	if err != nil {
		return fmt.Errorf("создание пула БД: %w", err)
	}
	defer dbPool.Close()

	pingCtx, pingCancel := context.WithTimeout(ctx, cfg.DatabaseConnectTimeout)
	defer pingCancel()

	if err := dbPool.Ping(pingCtx); err != nil {
		return fmt.Errorf("проверка подключения к БД: %w", err)
	}
	slog.Info("База данных подключена")

	tripRepository := repository.NewTripRepository(dbPool)
	txManager := repository.NewTxManager(dbPool, cfg.DatabaseQueryTimeout)
	tripService := trips.NewService(tripRepository, txManager)
	server := httpserver.NewServer(cfg, dbPool, tripService)
	defer server.Close()

	serverErr := make(chan error, 1)

	go func() {
		slog.Info("Запуск HTTP-сервера", "addr", server.Addr)
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("работа HTTP-сервера: %w", err)
		}
		return nil
	case <-ctx.Done():
		slog.Info("Получен сигнал завершения. Остановка сервера")

		// Лимит включает Shutdown и последующий defer dbPool.Close().
		go func() {
			timer := time.NewTimer(cfg.ShutdownTimeout)
			defer timer.Stop()
			select {
			case <-stopped:
				return
			case <-timer.C:
				slog.Error("Истёк таймаут остановки")
				os.Exit(1)
			}
		}()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("остановка HTTP-сервера: %w", err)
		}
	}

	slog.Info("Сервер остановлен")
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("Сервис завершён с ошибкой", "error", err)
		os.Exit(1)
	}
}

func createPool(ctx context.Context, config *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseUrl)
	if err != nil {
		return nil, fmt.Errorf("настройка пула: %w", err)
	}

	poolConfig.MaxConns = int32(config.DatabaseMaxConns)
	poolConfig.MinConns = int32(config.DatabaseMinConns)
	poolConfig.ConnConfig.ConnectTimeout = config.DatabaseConnectTimeout
	poolConfig.MaxConnLifetime = config.DatabaseMaxConnLifetime

	return pgxpool.NewWithConfig(ctx, poolConfig)
}
