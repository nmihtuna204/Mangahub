// Package main - gRPC Service Server
// Điểm vào cho gRPC server dùng cho inter-service communication
// Chức năng:
//   - High-performance RPC calls với Protocol Buffers
//   - GetManga, SearchManga, UpdateProgress RPCs
//   - Reflection API support cho debugging
//   - Audit logging và internal service calls
//
// Port: 9092
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcpkg "mangahub/internal/grpc"
	pb "mangahub/internal/grpc/pb"
	"mangahub/pkg/config"
	"mangahub/pkg/database"
	"mangahub/pkg/logger"
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

	addr := fmt.Sprintf("%s:%d", cfg.GRPC.Host, cfg.GRPC.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(100*1024*1024), // 100MB
		grpc.MaxSendMsgSize(100*1024*1024), // 100MB
	)
	mangaService := grpcpkg.NewMangaServiceServer(db.DB)
	pb.RegisterMangaServiceServer(grpcServer, mangaService)

	// Register reflection service for grpcurl
	reflection.Register(grpcServer)

	logger.Infof("gRPC server listening on %s", addr)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			logger.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	logger.Info("Shutting down gRPC server...")
	grpcServer.GracefulStop()
	logger.Info("gRPC server stopped.")
}
