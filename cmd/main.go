package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SteepTaq/avito-backend-internship-2025/internal/config"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/handler"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/repository"
	"github.com/SteepTaq/avito-backend-internship-2025/internal/service"
	"github.com/SteepTaq/avito-backend-internship-2025/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		os.Stderr.WriteString("Failed to load config: " + err.Error() + "\n")
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel)
	log.Info("Application starting", "version", "1.0.0")

	log.Info("Connecting to database")
	db, err := repository.InitDB(cfg.DatabaseURL)
	if err != nil {
		log.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}
	defer func() {
		db.Close()
		log.Info("Database connection pool closed")
	}()

	// слой репозитория
	repo := repository.NewRepository(db, log)

	// сервисный слой
	svc := service.NewService(repo, cfg.MaxReviewers, log)

	// Создаем handler
	h := handler.NewHandler(svc, cfg, log)

	// настраиваем маршруты
	router := h.SetupRoutes()

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info("Server starting", "port", cfg.ServerPort, "address", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// ожидаем сигнал для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("Server exited gracefully")
}
