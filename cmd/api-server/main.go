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
// Port: 8080
package main

import (
	"fmt"
	"log"
	"net/http"

	"mangahub/internal/auth"
	"mangahub/internal/chat"
	"mangahub/internal/comment"
	"mangahub/internal/leaderboard"
	"mangahub/internal/manga"
	"mangahub/internal/progress"
	"mangahub/internal/protocols"
	"mangahub/internal/rating"
	"mangahub/internal/udp"
	"mangahub/internal/websocket"
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

	// Initialize UDP server (for bridge)
	logger.Infof("Starting UDP notification server on %s:%d", cfg.UDP.Host, cfg.UDP.Port)
	udpServer := udp.NewNotificationServer(cfg.UDP.Host, cfg.UDP.Port)
	go func() {
		if err := udpServer.Start(); err != nil {
			logger.Errorf("UDP server error: %v", err)
		}
	}()

	// Initialize protocol bridge
	logger.Infof("Initializing protocol bridge (TCP:%d, gRPC:%d)", cfg.TCP.Port, cfg.GRPC.Port)
	protocolBridge, err := protocols.NewProtocolBridge(
		cfg.TCP.Host, cfg.TCP.Port,
		udpServer,
		cfg.GRPC.Host, cfg.GRPC.Port,
	)
	if err != nil {
		logger.Warnf("Protocol bridge initialization error: %v (will continue without bridge)", err)
	}
	if protocolBridge != nil {
		defer protocolBridge.Close()
	}

	authSvc := auth.NewService(db.DB, cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Expiration)
	authHandler := auth.NewHandler(authSvc)

	mangaRepo := manga.NewRepository(db.DB)
	mangaSvc := manga.NewService(mangaRepo)
	mangaHandler := manga.NewHandler(mangaSvc)

	progressRepo := progress.NewRepository(db.DB)
	progressSvc := progress.NewService(progressRepo)

	// Use bridge-enabled handler if bridge is available
	var progressHandler *progress.Handler
	if protocolBridge != nil {
		progressHandler = progress.NewHandlerWithBridge(progressSvc, protocolBridge)
		logger.Infof("Progress handler initialized with protocol bridge")
	} else {
		progressHandler = progress.NewHandler(progressSvc)
		logger.Warnf("Progress handler initialized without protocol bridge")
	}

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	wsHandler := websocket.NewHandler(wsHub)

	// ================================================
	// Phase 2: Social Features Initialization
	// Rating, Comment, Leaderboard, Chat persistence
	// ================================================

	// Initialize Chat repository for message persistence
	chatRepo := chat.NewRepository(db.DB)
	wsHub.SetChatRepository(chatRepo) // Enable chat persistence

	// Initialize Rating system
	ratingRepo := rating.NewRepository(db.DB)
	ratingSvc := rating.NewService(ratingRepo)
	ratingHandler := rating.NewHandler(ratingSvc)

	// Initialize Comment system
	commentRepo := comment.NewRepository(db.DB)
	commentSvc := comment.NewService(commentRepo)
	commentHandler := comment.NewHandler(commentSvc)

	// Initialize Leaderboard system
	leaderboardSvc := leaderboard.NewService(db.DB)
	leaderboardHandler := leaderboard.NewHandler(leaderboardSvc)

	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(logger.GinLogger(), logger.Recovery())

	api := router.Group("/")

	// Public auth routes
	api.POST("/auth/register", authHandler.Register)
	api.POST("/auth/login", authHandler.Login)

	// Public manga routes
	api.GET("/manga", mangaHandler.ListManga)
	api.GET("/manga/:id", mangaHandler.GetManga)

	// Health check endpoint
	api.GET("/health", func(c *gin.Context) {
		dbHealth, err := db.HealthCheck()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":   "unhealthy",
				"database": fmt.Sprintf("error: %v", err),
				"server":   "running",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":   "ok",
			"database": dbHealth,
			"server":   "running",
		})
	})

	protected := api.Group("/")
	protected.Use(auth.JWTMiddleware(authSvc))

	// Protected auth routes
	protected.GET("/auth/me", authHandler.GetMe)
	protected.POST("/auth/logout", authHandler.Logout)
	protected.POST("/auth/refresh", authHandler.RefreshToken)

	// Library endpoints
	protected.POST("/users/library", progressHandler.AddToLibrary)
	protected.GET("/users/library", progressHandler.GetLibrary)
	protected.DELETE("/users/library/:manga_id", progressHandler.RemoveFromLibrary)
	protected.PUT("/users/progress", progressHandler.UpdateProgress)

	// ================================================
	// Phase 2: Social Features Routes
	// ================================================

	// Rating routes (authenticated)
	// POST /manga/:id/ratings - Submit or update rating
	// DELETE /manga/:id/ratings - Delete user's rating
	protected.POST("/manga/:id/ratings", ratingHandler.SubmitRating)
	protected.DELETE("/manga/:id/ratings", ratingHandler.DeleteRating)

	// Rating routes (public - view only)
	// GET /manga/:id/ratings - Get ratings summary
	api.GET("/manga/:id/ratings", ratingHandler.GetRatings)

	// Comment routes (authenticated)
	// POST /manga/:id/comments - Create new comment
	// PUT /comments/:id - Update comment
	// DELETE /comments/:id - Delete comment
	// POST /comments/:id/like - Like comment
	// DELETE /comments/:id/like - Unlike comment
	protected.POST("/manga/:id/comments", commentHandler.CreateComment)
	protected.PUT("/comments/:id", commentHandler.UpdateComment)
	protected.DELETE("/comments/:id", commentHandler.DeleteComment)
	protected.POST("/comments/:id/like", commentHandler.LikeComment)
	protected.DELETE("/comments/:id/like", commentHandler.UnlikeComment)

	// Comment routes (public - view only)
	api.GET("/manga/:id/comments", commentHandler.GetComments)

	// Leaderboard routes (public)
	// GET /leaderboards/manga - Top rated manga
	// GET /leaderboards/users - Most active users  
	// GET /leaderboards/trending - Trending manga
	api.GET("/leaderboards/manga", leaderboardHandler.GetTopRatedManga)
	api.GET("/leaderboards/users", leaderboardHandler.GetMostActiveUsers)
	api.GET("/leaderboards/trending", leaderboardHandler.GetTrendingManga)

	// WebSocket chat endpoint (requires JWT)
	protected.GET("/ws/chat", wsHandler.ServeWS)

	// Room info endpoint
	api.GET("/rooms/:room_id", wsHandler.GetRoomInfo)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	logger.Infof("HTTP API server listening on %s", srv.Addr)
	logger.Infof("WebSocket chat available at ws://%s/ws/chat?room_id=<room>", srv.Addr)
	if protocolBridge != nil {
		logger.Infof("🔄 Phase 7: All 5 protocols integrated (HTTP + TCP + UDP + WebSocket + gRPC)")
	}
	logger.Infof("✨ Phase 2: Social features enabled (Rating, Comment, Leaderboard, Chat persistence)")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
}
