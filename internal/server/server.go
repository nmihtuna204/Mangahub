// Package server wires up the HTTP API: services, handlers, routes, the
// WebSocket hub and the protocol bridge. cmd/api-server and the end-to-end
// tests in test/ both build the API through New, so tests exercise exactly
// the routes and middleware that production serves.
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"mangahub/internal/activity"
	"mangahub/internal/auth"
	"mangahub/internal/chapters"
	"mangahub/internal/chat"
	"mangahub/internal/comment"
	"mangahub/internal/customlist"
	"mangahub/internal/leaderboard"
	"mangahub/internal/manga"
	"mangahub/internal/progress"
	"mangahub/internal/protocols"
	"mangahub/internal/ratelimit"
	"mangahub/internal/rating"
	"mangahub/internal/websocket"
	"mangahub/pkg/config"
	"mangahub/pkg/database"
	"mangahub/pkg/external"
	"mangahub/pkg/logger"
)

// Config holds what the API needs beyond the database.
type Config struct {
	JWT config.JWTConfig

	// Protocol bridge targets ("host:port"). The TCP, UDP and gRPC servers run
	// as separate processes; the bridge connects to them lazily.
	TCPAddr  string
	UDPAddr  string
	GRPCAddr string

	// Rate limits per client IP; 0 disables. Auth limits are per minute.
	RateLimit     float64
	RateBurst     int
	AuthRateLimit float64
	AuthRateBurst int

	// Background check for new chapters on MangaDex; 0 disables it
	ChapterSyncInterval time.Duration
	ChapterLanguage     string
	MangaDex            config.MangaDexConfig
}

// Server is the assembled HTTP API.
type Server struct {
	Router *gin.Engine
	Hub    *websocket.Hub
	Bridge *protocols.ProtocolBridge

	stopBackground context.CancelFunc
}

// New builds the API. Call Close when done to stop the hub and bridge.
func New(db *database.DB, cfg Config) *Server {
	// WebSocket hub (before the bridge, which notifies its chat rooms)
	wsHub := websocket.NewHub()
	wsHub.SetChatRepository(chat.NewRepository(db.DB))
	go wsHub.Run()
	wsHandler := websocket.NewHandler(wsHub)

	logger.Infof("Initializing protocol bridge (TCP %s, UDP %s, gRPC %s)", cfg.TCPAddr, cfg.UDPAddr, cfg.GRPCAddr)
	protocolBridge, err := protocols.NewProtocolBridge(cfg.TCPAddr, cfg.UDPAddr, cfg.GRPCAddr, wsHub)
	if err != nil {
		logger.Warnf("Protocol bridge initialization error: %v (will continue without bridge)", err)
	}

	authSvc := auth.NewService(db.DB, cfg.JWT.Secret, cfg.JWT.Issuer, cfg.JWT.Expiration)
	authHandler := auth.NewHandler(authSvc)

	mangaSvc := manga.NewService(manga.NewRepository(db.DB))
	mangaHandler := manga.NewHandler(mangaSvc)

	activitySvc := activity.NewService(activity.NewRepository(db.DB))
	activityHandler := activity.NewHandler(activitySvc)

	progressSvc := progress.NewService(progress.NewRepository(db.DB))
	var progressHandler *progress.Handler
	if protocolBridge != nil {
		progressHandler = progress.NewHandlerWithActivity(progressSvc, protocolBridge, activitySvc, mangaSvc)
	} else {
		progressHandler = progress.NewHandlerWithActivity(progressSvc, nil, activitySvc, mangaSvc)
	}

	ratingHandler := rating.NewHandler(rating.NewService(rating.NewRepository(db.DB))) // rating activity comes from DB triggers
	commentHandler := comment.NewHandler(comment.NewService(comment.NewRepository(db.DB)))
	leaderboardHandler := leaderboard.NewHandler(leaderboard.NewService(db.DB))

	// New chapters reach readers through the bridge: targeted UDP + chat room
	var notifier chapters.Notifier
	if protocolBridge != nil {
		notifier = protocolBridge
	}
	chapterSvc := chapters.NewService(db.DB, notifier)
	chapterHandler := chapters.NewHandler(chapterSvc)
	bgCtx, stopBackground := context.WithCancel(context.Background())
	if cfg.ChapterSyncInterval > 0 {
		source := chapters.NewMangaDexSource(external.NewMangaDexClient(&cfg.MangaDex), cfg.ChapterLanguage)
		go chapters.NewSyncer(db.DB, chapterSvc, source).RunEvery(bgCtx, cfg.ChapterSyncInterval)
		logger.Infof("New chapter sync with MangaDex every %v", cfg.ChapterSyncInterval)
	}
	listHandler := customlist.NewHandler(customlist.NewService(customlist.NewRepository(db.DB)))

	router := gin.New()
	// Client IP comes from the connection, not X-Forwarded-For, so clients
	// can't dodge the rate limiter with a made-up header
	_ = router.SetTrustedProxies(nil)
	router.Use(logger.GinLogger(), logger.Recovery(), cors(),
		ratelimit.New(cfg.RateLimit, cfg.RateBurst).Middleware())

	api := router.Group("/")

	// Public auth routes, with a stricter limit against password guessing
	// Registration counts every attempt; login counts only wrong passwords
	// (401), so password guessing is limited but normal logins never are
	registerLimit := ratelimit.PerMinute(cfg.AuthRateLimit, cfg.AuthRateBurst).Middleware()
	loginLimit := ratelimit.PerMinute(cfg.AuthRateLimit, cfg.AuthRateBurst).
		FailureMiddleware(func(status int) bool { return status == http.StatusUnauthorized })
	api.POST("/auth/register", registerLimit, authHandler.Register)
	api.POST("/auth/login", loginLimit, authHandler.Login)

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
			"status":   "healthy",
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

	// Activity Feed routes
	api.GET("/activities", activityHandler.GetRecentActivities)
	protected.GET("/activities/user/:userID", activityHandler.GetUserActivities)

	// Rating routes: submit/delete need auth, viewing is public
	protected.POST("/manga/:id/ratings", ratingHandler.SubmitRating)
	protected.DELETE("/manga/:id/ratings", ratingHandler.DeleteRating)
	api.GET("/manga/:id/ratings", ratingHandler.GetRatings)

	// Comment routes (authenticated)
	protected.POST("/manga/:id/comments", commentHandler.CreateComment)
	protected.PUT("/comments/:id", commentHandler.UpdateComment)
	protected.DELETE("/comments/:id", commentHandler.DeleteComment)
	protected.POST("/comments/:id/like", commentHandler.LikeComment)
	protected.DELETE("/comments/:id/like", commentHandler.UnlikeComment)

	// Comment routes (public - view only; a token, if sent, fills in liked_by_me)
	api.GET("/manga/:id/comments", auth.OptionalAuthMiddleware(authSvc), commentHandler.GetComments)

	// Custom lists: reading is public for public lists (a token, if sent,
	// also shows the caller's private lists); changes need the owner
	optionalAuth := auth.OptionalAuthMiddleware(authSvc)
	api.GET("/lists", optionalAuth, listHandler.GetLists)
	api.GET("/lists/:id", optionalAuth, listHandler.GetList)
	protected.POST("/lists", listHandler.CreateList)
	protected.PUT("/lists/:id", listHandler.UpdateList)
	protected.DELETE("/lists/:id", listHandler.DeleteList)
	protected.POST("/lists/:id/items", listHandler.AddItem)
	protected.DELETE("/lists/:id/items/:manga_id", listHandler.RemoveItem)
	protected.PUT("/lists/:id/order", listHandler.ReorderItems)

	// Admin: release a new chapter by hand (the background sync does the same)
	admin := protected.Group("/admin", auth.RequireRole("admin"))
	admin.POST("/manga/:id/chapters", chapterHandler.ReleaseChapter)

	// Leaderboard routes (public)
	api.GET("/leaderboards/manga", leaderboardHandler.GetTopRatedManga)
	api.GET("/leaderboards/users", leaderboardHandler.GetMostActiveUsers)
	api.GET("/leaderboards/trending", leaderboardHandler.GetTrendingManga)

	// WebSocket chat endpoint (requires JWT via Authorization header or ?token=)
	api.GET("/ws/chat", auth.WSAuthMiddleware(authSvc), wsHandler.ServeWS)

	// Room info + persisted history
	api.GET("/rooms/:room_id", wsHandler.GetRoomInfo)
	api.GET("/rooms/:room_id/messages", wsHandler.GetRoomMessages)

	return &Server{Router: router, Hub: wsHub, Bridge: protocolBridge, stopBackground: stopBackground}
}

// cors lets browser clients on other origins call the API. Allowing any
// origin is safe because auth uses bearer tokens, not cookies: the browser
// never attaches credentials on its own, so a foreign page can't act as the user.
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		h.Set("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent) // preflight
			return
		}
		c.Next()
	}
}

// Close stops background work, the protocol bridge and the WebSocket hub.
func (s *Server) Close() {
	s.stopBackground()
	if s.Bridge != nil {
		_ = s.Bridge.Close()
	}
	s.Hub.Stop()
}
