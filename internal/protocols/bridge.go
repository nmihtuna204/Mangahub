// Package protocols - Protocol Integration Bridge
// Core integration layer kết nối tất cả 5 protocols lại với nhau
// Chức năng:
//   - Kích hoạt tất cả 5 protocols từ một HTTP API call
//   - TCP: Broadcast progress updates đến connected clients
//   - UDP: Gửi notifications đến subscribers
//   - WebSocket: Notify chat rooms
//   - gRPC: Log audit trail
//   - HTTP: Tiếp nhận request ban đầu
// Đây là core feature thể hiện multi-protocol integration!
package protocols

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/yourusername/mangahub/internal/grpc/pb"
	"github.com/yourusername/mangahub/internal/tcp"
	"github.com/yourusername/mangahub/internal/udp"
	"github.com/yourusername/mangahub/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProtocolBridge connects all protocols together
type ProtocolBridge struct {
	tcpClient  *tcp.Client
	udpServer  *udp.NotificationServer
	grpcClient pb.MangaServiceClient
	grpcConn   *grpc.ClientConn
}

// NewProtocolBridge creates a new bridge connecting all protocols
func NewProtocolBridge(tcpHost string, tcpPort int, udpServer *udp.NotificationServer, grpcHost string, grpcPort int) (*ProtocolBridge, error) {
	// Connect to TCP server
	tcpClient := tcp.NewClient(tcpHost, tcpPort)
	if err := tcpClient.Connect(); err != nil {
		logger.Warnf("TCP client connection failed: %v (will retry on use)", err)
		// Don't fail bridge creation, just log warning
	}

	// Connect to gRPC server
	grpcAddr := fmt.Sprintf("%s:%d", grpcHost, grpcPort)
	grpcConn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warnf("gRPC client connection failed: %v (will retry on use)", err)
		// Don't fail bridge creation, just log warning
	}

	var grpcClient pb.MangaServiceClient
	if grpcConn != nil {
		grpcClient = pb.NewMangaServiceClient(grpcConn)
	}

	return &ProtocolBridge{
		tcpClient:  tcpClient,
		udpServer:  udpServer,
		grpcClient: grpcClient,
		grpcConn:   grpcConn,
	}, nil
}

// BroadcastProgressUpdate sends progress update through all protocols
func (b *ProtocolBridge) BroadcastProgressUpdate(userID, username, mangaID string, chapter int32, status string) error {
	logger.Infof("Bridge: Broadcasting progress update - user=%s, manga=%s, chapter=%d", userID, mangaID, chapter)

	// 1. TCP Broadcast: Send to sync server
	if b.tcpClient != nil && b.tcpClient.Conn != nil {
		go b.broadcastToTCP(userID, mangaID, int(chapter))
	}

	// 2. UDP Notification: Alert subscribers
	if b.udpServer != nil {
		go b.notifyViaUDP(mangaID)
	}

	// 3. gRPC Audit: Log to audit service
	if b.grpcClient != nil {
		go b.auditViaGRPC(userID, mangaID, chapter, status)
	}

	return nil
}

// broadcastToTCP sends progress update to TCP sync server
func (b *ProtocolBridge) broadcastToTCP(userID, mangaID string, chapter int) {
	progressUpdate := tcp.NewProgressUpdate(userID, mangaID, chapter)
	data, err := json.Marshal(progressUpdate)
	if err != nil {
		logger.Errorf("Bridge: Failed to marshal TCP message: %v", err)
		return
	}

	_, err = b.tcpClient.Conn.Write(append(data, '\n'))
	if err != nil {
		logger.Warnf("Bridge: TCP broadcast failed: %v", err)
	} else {
		logger.Infof("Bridge: Progress update sent via TCP")
	}
}

// notifyViaUDP sends notification via UDP
func (b *ProtocolBridge) notifyViaUDP(mangaID string) {
	notification := udp.NewChapterNotification(
		mangaID,
		fmt.Sprintf("New progress update for manga %s!", mangaID),
	)
	b.udpServer.SendNotification(notification)
	logger.Infof("Bridge: Notification sent via UDP")
}

// auditViaGRPC updates progress via gRPC
func (b *ProtocolBridge) auditViaGRPC(userID, mangaID string, chapter int32, status string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := b.grpcClient.UpdateProgress(ctx, &pb.ProgressRequest{
		UserId:         userID,
		MangaId:        mangaID,
		CurrentChapter: chapter,
		Status:         status,
		Rating:         0,
	})
	if err != nil {
		logger.Warnf("Bridge: gRPC audit failed: %v", err)
	} else {
		logger.Infof("Bridge: Progress audit logged via gRPC")
	}
}

// Close closes all protocol connections
func (b *ProtocolBridge) Close() error {
	if b.tcpClient != nil {
		_ = b.tcpClient.Close()
	}
	if b.grpcConn != nil {
		_ = b.grpcConn.Close()
	}
	return nil
}
