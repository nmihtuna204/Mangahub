// Package main - HTTP REST API Server
// Điểm vào chính cho HTTP REST API server
// Chức năng:
//   - Xử lý HTTP requests (GET, POST, PUT, DELETE)
//   - Quản lý user authentication với JWT
//   - API endpoints cho manga search, library management
//   - Tích hợp với tất cả 5 protocols thông qua Protocol Bridge
//   - WebSocket chat server endpoint
//   - Phase 2: Rating, Comment, Leaderboard APIs
//
// Routes and wiring live in internal/server.
//
// Port: 8080
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mangahub/internal/server"
	"mangahub/pkg/config"
	"mangahub/pkg/database"
	"mangahub/pkg/logger"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := config.Load("./configs/development.yaml")
	if err != nil {
		log.Fatal("failed to load config:", err)
	}

	logger.Init(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	// Deferred first, so it runs last: after api.Close and db.Close
	defer logger.Info("API server stopped.")

	db, err := database.NewDB(database.Config{
		Path:            cfg.Database.Path,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		logger.Fatal("failed to init database:", err)
	}
	defer db.Close()

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	api := server.New(db, server.Config{
		JWT:      cfg.JWT,
		TCPAddr:  fmt.Sprintf("%s:%d", cfg.TCP.Host, cfg.TCP.Port),
		UDPAddr:  fmt.Sprintf("%s:%d", cfg.UDP.Host, cfg.UDP.Port),
		GRPCAddr: fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port),

		RateLimit:     cfg.Server.RateLimit,
		RateBurst:     cfg.Server.RateBurst,
		AuthRateLimit: cfg.Server.AuthRateLimit,
		AuthRateBurst: cfg.Server.AuthRateBurst,

		ChapterSyncInterval: cfg.Chapters.SyncInterval,
		ChapterLanguage:     cfg.Chapters.Language,
		MangaDex:            cfg.MangaDex,
	})
	defer api.Close()

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      api.Router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	logger.Infof("HTTP API server listening on %s", srv.Addr)
	logger.Infof("WebSocket chat available at ws://%s/ws/chat?room_id=<room>", srv.Addr)
	logger.Infof("🔄 All 5 protocols integrated (HTTP + TCP + UDP + WebSocket + gRPC)")
	logger.Infof("✨ Social features enabled (Rating, Comment, Leaderboard, Chat persistence)")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// A listen error (e.g. port in use) ends main normally, so the deferred
	// cleanup still runs (logger.Fatalf here would skip it)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()

	select {
	case err := <-serveErr:
		logger.Errorf("server error: %v", err)
		return
	case sig := <-sigCh:
		logger.Infof("Received %v, shutting down API server (press Ctrl+C again to force quit)...", sig)
	}
	// A second Ctrl+C now kills the process the default way
	signal.Stop(sigCh)

	// Graceful shutdown: stop accepting connections and finish in-flight
	// requests; then the deferred api.Close (background jobs, protocol
	// bridge queues, WebSocket clients get a close frame, queued chat
	// messages saved) and db.Close run, in that order
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Errorf("error during shutdown: %v", err)
	}
	logger.Info("HTTP server stopped; closing hub, bridge and database...")
}
