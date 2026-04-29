// Package main bootstraps the API server for OmniLedger.
//
// Initialization sequence: configuration, structured logger, Postgres,
// Redis, email worker pool, rate limiter, dependency wiring, HTTP
// routing, CORS, and graceful shutdown.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"

	"github.com/akdandapat/OmniLedger/internal/auth"
	"github.com/akdandapat/OmniLedger/internal/config"
	"github.com/akdandapat/OmniLedger/internal/email"
	"github.com/akdandapat/OmniLedger/internal/handler"
	"github.com/akdandapat/OmniLedger/internal/logger"
	"github.com/akdandapat/OmniLedger/internal/middleware"
	"github.com/akdandapat/OmniLedger/internal/service"
	"github.com/akdandapat/OmniLedger/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(cfg.Env)
	slog.Info("configuration loaded", "env", cfg.Env, "port", cfg.Port)

	db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
	if err != nil {
		slog.Error("postgres connection failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	slog.Info("connected to postgres")

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis connection failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()
	slog.Info("connected to redis")

	emailWorker := email.NewWorker(email.SMTPConfig{
		Host:     cfg.SMTPHost,
		Port:     cfg.SMTPPort,
		Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword,
		From:     cfg.SMTPFrom,
	}, 3, 100)
	emailWorker.Start(3)
	defer emailWorker.Shutdown()

	rateLimiter := middleware.NewRateLimiter(rdb, middleware.RateLimiterConfig{
		MaxRequests: 5,
		Window:      1 * time.Minute,
	})
	slog.Info("rate limiter initialized", "max_requests", 5, "window", "1m")

	dataStore := store.New(db)

	tokenMgr := auth.NewTokenManager(cfg.JWTSecret, rdb)
	authMiddleware := auth.NewMiddleware(tokenMgr, db)
	authSvc := service.NewAuthService(dataStore, tokenMgr)

	transferSvc := service.NewTransferService(dataStore, rdb)
	fundingSvc := service.NewFundingService(dataStore)

	isSecure := cfg.Env == "production"
	authHandler := handler.NewAuthHandler(authSvc, isSecure)
	transferHandler := handler.NewTransferHandler(transferSvc)
	fundingHandler := handler.NewFundingHandler(fundingSvc, db)
	accountHandler := handler.NewAccountHandler(dataStore)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"omniledger"}`)
	})

	authHandler.RegisterAuthRoutes(mux)
	mux.Handle("POST /api/v1/auth/login",
		rateLimiter.Limit("auth:login",
			http.HandlerFunc(authHandler.HandleLogin)))
	mux.Handle("POST /api/v1/auth/register",
		rateLimiter.Limit("auth:register",
			http.HandlerFunc(authHandler.HandleRegister)))

	mux.Handle("POST /api/v1/transfers",
		rateLimiter.Limit("transfers",
			authMiddleware.Protect(
				http.HandlerFunc(transferHandler.ServeTransfer))))

	accountHandler.RegisterAccountRoutes(mux, authMiddleware)
	fundingHandler.RegisterFundingRoutes(mux, authMiddleware)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{cfg.CORSOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	}).Handler(mux)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      corsHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		slog.Info("server started", "addr", ":"+cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("initiating shutdown")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
