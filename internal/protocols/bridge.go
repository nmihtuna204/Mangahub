// Package protocols - Protocol Integration Bridge
// Core integration layer kết nối tất cả 5 protocols lại với nhau
// Chức năng:
//   - Kích hoạt tất cả 5 protocols từ một HTTP API call
//   - TCP: Broadcast progress updates đến connected clients
//   - UDP: Gửi notifications đến subscribers (qua standalone UDP server)
//   - WebSocket: Notify chat room của manga
//   - gRPC: Log audit trail
//   - HTTP: Tiếp nhận request ban đầu
//
// Đây là core feature thể hiện multi-protocol integration!
package protocols

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	pb "mangahub/internal/grpc/pb"
	"mangahub/internal/tcp"
	"mangahub/internal/udp"
	"mangahub/pkg/logger"
)

const (
	dialTimeout  = 2 * time.Second
	writeTimeout = 2 * time.Second
	grpcTimeout  = 5 * time.Second
	grpcQueueLen = 256
)

// ProgressEvent is a completed progress update to fan out to every protocol.
type ProgressEvent struct {
	UserID     string
	Username   string
	MangaID    string
	MangaTitle string
	Chapter    int32
	Status     string
	// Token is the user's JWT from the HTTP request; the gRPC audit call
	// forwards it, since UpdateProgress requires an authenticated caller
	Token string
}

// RoomNotifier delivers a server-generated message to a WebSocket chat room.
// Implemented by websocket.Hub.
type RoomNotifier interface {
	NotifyRoom(roomID, userID, username, content, msgType string)
}

// MangaRoomID is the chat room ID the TUI uses for a manga's discussion room.
func MangaRoomID(mangaID string) string {
	return "manga_" + mangaID
}

// ProtocolBridge connects all protocols together.
// TCP and UDP connections are dialed lazily and redialed after failures, so the
// bridge recovers when a server starts late or restarts.
type ProtocolBridge struct {
	tcpAddr string
	tcpMu   sync.Mutex
	tcpConn net.Conn

	udpAddr string
	udpMu   sync.Mutex
	udpConn net.Conn

	grpcConn   *grpc.ClientConn
	grpcClient pb.MangaServiceClient
	grpcQueue  chan auditCall

	rooms RoomNotifier

	closeOnce sync.Once
	done      chan struct{}
}

// NewProtocolBridge creates a new bridge connecting all protocols.
// rooms may be nil, in which case the WebSocket leg is skipped.
func NewProtocolBridge(tcpAddr, udpAddr, grpcAddr string, rooms RoomNotifier) (*ProtocolBridge, error) {
	b := &ProtocolBridge{
		tcpAddr:   tcpAddr,
		udpAddr:   udpAddr,
		grpcQueue: make(chan auditCall, grpcQueueLen),
		rooms:     rooms,
		done:      make(chan struct{}),
	}

	// Connect eagerly so the startup log shows whether TCP is reachable;
	// a failure here is retried on every broadcast.
	b.tcpMu.Lock()
	if err := b.dialTCPLocked(); err != nil {
		logger.Warnf("Bridge: TCP server not reachable yet: %v (will retry on use)", err)
	}
	b.tcpMu.Unlock()

	// grpc.NewClient does not connect until the first RPC, and reconnects on its own.
	grpcConn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Warnf("Bridge: gRPC client setup failed: %v (gRPC leg disabled)", err)
	} else {
		b.grpcConn = grpcConn
		b.grpcClient = pb.NewMangaServiceClient(grpcConn)
		go b.grpcWorker()
	}

	return b, nil
}

// BroadcastProgressUpdate sends a progress update through all protocols.
// Each leg only logs on failure; it never fails the originating HTTP request.
func (b *ProtocolBridge) BroadcastProgressUpdate(ev ProgressEvent) error {
	logger.Infof("Bridge: Broadcasting progress update - user=%s, manga=%s, chapter=%d", ev.UserID, ev.MangaID, ev.Chapter)

	// 1. TCP: push to the sync server, which relays to every connected client
	go b.broadcastToTCP(ev)

	// 2. UDP: ask the standalone UDP server to notify its subscribers
	go b.notifyViaUDP(ev)

	// 3. gRPC: queued so audit writes reach the server in the same order as the HTTP updates
	if b.grpcClient != nil {
		b.enqueueGRPC(ev)
	}

	// 4. WebSocket: tell the manga's chat room
	if b.rooms != nil {
		b.rooms.NotifyRoom(MangaRoomID(ev.MangaID), ev.UserID, ev.Username, describe(ev), "system")
		logger.Infof("Bridge: Chat room %s notified via WebSocket", MangaRoomID(ev.MangaID))
	}

	return nil
}

func describe(ev ProgressEvent) string {
	title := ev.MangaTitle
	if title == "" {
		title = ev.MangaID
	}
	if ev.Status == "completed" {
		return fmt.Sprintf("%s finished %s", ev.Username, title)
	}
	return fmt.Sprintf("%s is now on chapter %d of %s", ev.Username, ev.Chapter, title)
}

// ================================
// TCP leg
// ================================

func (b *ProtocolBridge) broadcastToTCP(ev ProgressEvent) {
	data, err := json.Marshal(tcp.NewProgressUpdate(ev.UserID, ev.MangaID, int(ev.Chapter)))
	if err != nil {
		logger.Errorf("Bridge: Failed to marshal TCP message: %v", err)
		return
	}
	if err := b.sendTCP(append(data, '\n')); err != nil {
		logger.Warnf("Bridge: TCP broadcast failed: %v", err)
		return
	}
	logger.Infof("Bridge: Progress update sent via TCP")
}

// sendTCP writes to the sync server, dialing or redialing as needed.
func (b *ProtocolBridge) sendTCP(data []byte) error {
	b.tcpMu.Lock()
	defer b.tcpMu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if b.tcpConn == nil {
			if err := b.dialTCPLocked(); err != nil {
				return err
			}
		}
		_ = b.tcpConn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := b.tcpConn.Write(data); err != nil {
			lastErr = err
			_ = b.tcpConn.Close()
			b.tcpConn = nil
			continue // stale connection (e.g. server restarted) - redial once
		}
		return nil
	}
	return lastErr
}

func (b *ProtocolBridge) dialTCPLocked() error {
	conn, err := net.DialTimeout("tcp", b.tcpAddr, dialTimeout)
	if err != nil {
		return err
	}
	b.tcpConn = conn
	logger.Infof("Bridge: TCP connected to %s", b.tcpAddr)

	// The sync server relays every update to all clients, including us.
	// Drain those so our receive buffer never fills, and drop the connection
	// as soon as the server goes away so the next send redials.
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		b.tcpMu.Lock()
		if b.tcpConn == conn {
			_ = conn.Close()
			b.tcpConn = nil
		}
		b.tcpMu.Unlock()
	}()
	return nil
}

// ================================
// UDP leg
// ================================

func (b *ProtocolBridge) notifyViaUDP(ev ProgressEvent) {
	n := udp.NewProgressNotification(ev.MangaID, ev.MangaTitle, ev.Username, int(ev.Chapter), describe(ev))
	payload, err := json.Marshal(n)
	if err != nil {
		logger.Errorf("Bridge: Failed to marshal UDP notification: %v", err)
		return
	}
	if err := b.sendUDP(append([]byte("BROADCAST "), payload...)); err != nil {
		logger.Warnf("Bridge: UDP notification failed: %v", err)
		return
	}
	logger.Infof("Bridge: Notification sent via UDP")
}

// NotifyChapterRelease announces a new chapter to its readers (a
// chapters.Notifier): a UDP notification delivered only to those users, and a
// system notice in the manga's chat room.
func (b *ProtocolBridge) NotifyChapterRelease(ctx context.Context, mangaID, mangaTitle string, chapter int, userIDs []string) error {
	if b.rooms != nil {
		b.rooms.NotifyRoom(MangaRoomID(mangaID), "", "MangaHub",
			fmt.Sprintf("📖 Chapter %d of %s is out!", chapter, mangaTitle), "system")
	}
	for _, n := range udp.ChapterReleaseBatches(mangaID, mangaTitle, chapter, userIDs) {
		datagram, err := udp.BroadcastRequest(n)
		if err != nil {
			return err
		}
		if err := b.sendUDP(datagram); err != nil {
			return fmt.Errorf("chapter release notification: %w", err)
		}
	}
	logger.Infof("Bridge: Chapter %d of %s announced to %d readers via UDP", chapter, mangaTitle, len(userIDs))
	return nil
}

func (b *ProtocolBridge) sendUDP(datagram []byte) error {
	b.udpMu.Lock()
	defer b.udpMu.Unlock()

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if b.udpConn == nil {
			conn, err := net.Dial("udp", b.udpAddr)
			if err != nil {
				return err
			}
			b.udpConn = conn
		}
		_ = b.udpConn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := b.udpConn.Write(datagram); err != nil {
			// A connected UDP socket reports an earlier ICMP "port unreachable"
			// on the next write; reopen and try again.
			lastErr = err
			_ = b.udpConn.Close()
			b.udpConn = nil
			continue
		}
		return nil
	}
	return lastErr
}

// ================================
// gRPC leg
// ================================

// auditCall is one queued UpdateProgress audit and the token to send it with
type auditCall struct {
	req   *pb.ProgressRequest
	token string
}

func (b *ProtocolBridge) enqueueGRPC(ev ProgressEvent) {
	call := auditCall{
		req: &pb.ProgressRequest{
			UserId:         ev.UserID,
			MangaId:        ev.MangaID,
			CurrentChapter: ev.Chapter,
			Status:         ev.Status,
		},
		token: ev.Token,
	}
	select {
	case b.grpcQueue <- call:
	default:
		logger.Warnf("Bridge: gRPC audit queue full, dropping update for manga=%s", ev.MangaID)
	}
}

// grpcAuditKey must match grpc.AuditMetadataKey in internal/grpc: it tells the
// server this update is already saved, so it logs an audit entry instead of
// writing (a write here could land after a newer HTTP update and roll it back).
const grpcAuditKey = "x-mangahub-audit"

// grpcWorker sends audit RPCs one at a time, in the order the updates happened.
func (b *ProtocolBridge) grpcWorker() {
	for {
		select {
		case call := <-b.grpcQueue:
			ctx, cancel := context.WithTimeout(context.Background(), grpcTimeout)
			ctx = metadata.AppendToOutgoingContext(ctx, grpcAuditKey, "true", "authorization", "Bearer "+call.token)
			_, err := b.grpcClient.UpdateProgress(ctx, call.req)
			cancel()
			if err != nil {
				logger.Warnf("Bridge: gRPC audit failed: %v", err)
			} else {
				logger.Infof("Bridge: Progress audit logged via gRPC")
			}
		case <-b.done:
			return
		}
	}
}

// Close closes all protocol connections
func (b *ProtocolBridge) Close() error {
	b.closeOnce.Do(func() {
		close(b.done)

		b.tcpMu.Lock()
		if b.tcpConn != nil {
			_ = b.tcpConn.Close()
			b.tcpConn = nil
		}
		b.tcpMu.Unlock()

		b.udpMu.Lock()
		if b.udpConn != nil {
			_ = b.udpConn.Close()
			b.udpConn = nil
		}
		b.udpMu.Unlock()

		if b.grpcConn != nil {
			_ = b.grpcConn.Close()
		}
	})
	return nil
}
