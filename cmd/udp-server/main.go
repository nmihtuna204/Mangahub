// Package main - UDP Notification Server
// Điểm vào cho UDP server dùng để gửi push notifications
// Chức năng:
//   - Nhận datagram từ clients (REGISTER/UNREGISTER)
//   - Gửi chapter release notifications đến subscribers
//   - Connectionless protocol - không cần maintain connections
//   - Broadcast notifications đến nhiều clients
//
// Port: 9091
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mangahub/internal/auth"
	"mangahub/internal/udp"
	"mangahub/pkg/config"
	"mangahub/pkg/logger"
)

func main() {
	demo := flag.Bool("demo", false, "broadcast a sample chapter notification every 10s")
	flag.Parse()

	cfg, err := config.Load("./configs/development.yaml")
	if err != nil {
		panic(err)
	}

	logger.Init(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	server := udp.NewNotificationServer(cfg.UDP.Host, cfg.UDP.Port)
	// "REGISTER <jwt>" ties a subscriber to its user, so chapter releases
	// reach only the readers of that manga
	server.SetTokenVerifier(func(token string) (string, error) {
		user, err := auth.VerifyToken(token, []byte(cfg.JWT.Secret), cfg.JWT.Issuer)
		if err != nil {
			return "", err
		}
		return user.ID, nil
	})
	server.SetSubscriberTTL(cfg.UDP.SubscriberTTL)

	// Start server in background
	go func() {
		if err := server.Start(); err != nil {
			logger.Fatalf("UDP server error: %v", err)
		}
	}()

	logger.Infof("UDP Notification Server started on %s:%d", cfg.UDP.Host, cfg.UDP.Port)

	// Demo: Send test notifications periodically (opt-in; real notifications
	// come from the API server's protocol bridge on every progress update)
	if *demo {
		go func() {
			time.Sleep(5 * time.Second) // Wait for clients to register
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				notification := udp.NewChapterNotification(
					"one-piece",
					"New chapter released: One Piece Chapter 1100!",
				)
				server.SendNotification(notification)
				logger.Info("Demo notification sent")
			}
		}()
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down UDP server...")
	if err := server.Stop(); err != nil {
		logger.Errorf("error stopping UDP server: %v", err)
	}
	logger.Info("UDP server stopped.")
}
