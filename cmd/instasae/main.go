package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/italomoia/instasae/internal/cache"
	"github.com/italomoia/instasae/internal/client"
	"github.com/italomoia/instasae/internal/config"
	"github.com/italomoia/instasae/internal/crypto"
	"github.com/italomoia/instasae/internal/handler"
	"github.com/italomoia/instasae/internal/repository"
	"github.com/italomoia/instasae/internal/server"
	"github.com/italomoia/instasae/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel)
	logger := slog.Default()
	slog.Info("config loaded")

	ctx := context.Background()

	// PostgreSQL
	pgCtx, pgCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pgCancel()

	pool, err := pgxpool.New(pgCtx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to create postgres pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(pgCtx); err != nil {
		slog.Error("failed to ping postgres", "error", err)
		os.Exit(1)
	}
	slog.Info("postgres connected")

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("failed to parse redis url", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	redisCtx, redisCancel := context.WithTimeout(ctx, 5*time.Second)
	defer redisCancel()

	if err := redisClient.Ping(redisCtx).Err(); err != nil {
		slog.Error("failed to ping redis", "error", err)
		os.Exit(1)
	}
	slog.Info("redis connected")

	// Crypto
	enc, err := crypto.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		slog.Error("failed to create encryptor", "error", err)
		os.Exit(1)
	}

	// Repositories
	accountRepo := repository.NewAccountRepo(pool, enc)
	contactRepo := repository.NewContactRepo(pool)
	convRepo := repository.NewConversationRepo(pool)

	// Cache
	redisCache := cache.NewRedisCache(redisClient)

	// External clients
	httpClient := &http.Client{Timeout: 30 * time.Second}
	igClient := client.NewIGClient(httpClient, cfg.MetaGraphAPIVersion, cfg.EnableHumanAgentTag)
	cwClient := client.NewCWClient(httpClient)
	b2Client := client.NewB2Storage(client.B2Config{
		Endpoint:       cfg.B2Endpoint,
		Region:         cfg.B2Region,
		Bucket:         cfg.B2Bucket,
		KeyID:          cfg.B2KeyID,
		ApplicationKey: cfg.B2ApplicationKey,
		PublicURL:      cfg.B2PublicURL,
		Prefix:         cfg.B2Prefix,
	})

	// Services
	mediaSvc := service.NewMediaService(b2Client, httpClient)
	accountSvc := service.NewAccountService(accountRepo, redisCache)
	igSvc := service.NewInstagramService(
		accountRepo, contactRepo, convRepo,
		igClient, cwClient, redisCache, mediaSvc,
		logger, cfg.MetaAppSecret,
	)
	cwSvc := service.NewChatwootService(
		accountRepo, contactRepo, convRepo,
		igClient, cwClient, redisCache,
		logger,
	)

	// Background jobs
	tokenChecker := service.NewTokenChecker(accountRepo, logger, 6*time.Hour, 7*24*time.Hour)
	tokenChecker.Start(ctx)

	// Handlers
	handlers := server.Handlers{
		WebhookInstagram: handler.NewWebhookInstagramHandler(igSvc, cfg.WebhookVerifyToken),
		WebhookChatwoot:  handler.NewWebhookChatwootHandler(cwSvc),
		AdminAccounts:    handler.NewAdminAccountsHandler(accountSvc),
		Health:           handler.NewHealthHandler(pool, redisClient),
	}

	// HTTP Server
	srv := server.NewServer(cfg, handlers, logger)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("server started", "port", cfg.Port)

	// Graceful shutdown
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-sigCtx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}

func setupLogger(level string) {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}
