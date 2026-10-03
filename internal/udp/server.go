// Package udp - UDP Notification Server Implementation
// Quản lý UDP datagram communication cho push notifications
// Chức năng:
//   - Nhận REGISTER [token] / UNREGISTER messages từ clients
//   - Maintain subscriber list (with the user behind each one, when known)
//   - Broadcast notifications đến tất cả subscribers, or only to the users a
//     notification targets (e.g. chapter releases for manga in their library)
//   - Connectionless protocol - không maintain state
//   - JSON datagram format
//   - Non-blocking sends
package udp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"mangahub/pkg/logger"
)

// maxDatagram is the largest UDP payload; broadcast requests carrying a list
// of target user IDs can be well over a few KB
const maxDatagram = 65507

// DefaultSubscriberTTL is how long a subscriber stays registered without
// re-sending REGISTER. Clients re-register every HeartbeatInterval.
const (
	DefaultSubscriberTTL = 5 * time.Minute
	HeartbeatInterval    = time.Minute
)

// TokenVerifier turns a JWT into the user ID it belongs to.
type TokenVerifier func(token string) (userID string, err error)

// subscriber is a registered address and, if it registered with a token, its user
type subscriber struct {
	addr     *net.UDPAddr
	userID   string
	lastSeen time.Time
}

// NotificationServer manages UDP notification broadcasting
type NotificationServer struct {
	Addr       string
	connMu     sync.Mutex // Serve sets conn while Stop may run on another goroutine
	conn       *net.UDPConn
	clientsMu  sync.RWMutex
	clients    map[string]subscriber // clientID (address) -> subscriber
	Broadcast  chan Notification
	register   chan subscriber
	unregister chan string
	stop       chan struct{}
	verify     TokenVerifier
	ttl        time.Duration
}

// NewNotificationServer creates a new UDP notification server
func NewNotificationServer(host string, port int) *NotificationServer {
	return &NotificationServer{
		Addr:       fmt.Sprintf("%s:%d", host, port),
		clients:    make(map[string]subscriber),
		Broadcast:  make(chan Notification, 100),
		register:   make(chan subscriber),
		unregister: make(chan string),
		stop:       make(chan struct{}),
		ttl:        DefaultSubscriberTTL,
	}
}

// SetSubscriberTTL changes how long a silent subscriber is kept (default
// DefaultSubscriberTTL). Call before Serve.
func (s *NotificationServer) SetSubscriberTTL(ttl time.Duration) {
	if ttl > 0 {
		s.ttl = ttl
	}
}

// SetTokenVerifier enables "REGISTER <token>", which ties a subscriber to a
// user so it can receive notifications targeted at that user. Call before Serve.
func (s *NotificationServer) SetTokenVerifier(v TokenVerifier) {
	s.verify = v
}

// Start starts the UDP notification server
func (s *NotificationServer) Start() error {
	addr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		return fmt.Errorf("resolve udp addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	return s.Serve(conn)
}

// Serve handles registrations and broadcasts on an existing socket until Stop is called.
func (s *NotificationServer) Serve(conn *net.UDPConn) error {
	s.connMu.Lock()
	s.conn = conn
	s.connMu.Unlock()

	logger.Infof("UDP Notification Server listening on %s", conn.LocalAddr())

	go s.runHub()
	go s.listenForRegistrations()

	<-s.stop
	return nil
}

// runHub manages client registration and broadcasting
func (s *NotificationServer) runHub() {
	sweep := time.NewTicker(s.ttl / 2)
	defer sweep.Stop()

	for {
		select {
		case sub := <-s.register:
			// REGISTER doubles as the heartbeat: re-registering refreshes lastSeen
			clientID := sub.addr.String()
			sub.lastSeen = time.Now()
			s.clientsMu.Lock()
			_, known := s.clients[clientID]
			s.clients[clientID] = sub
			total := len(s.clients)
			s.clientsMu.Unlock()
			if !known {
				// Protocol trace logging
				logger.UDP("REGISTER", clientID, fmt.Sprintf("user=%s total_subscribers=%d", sub.userID, total))
			}

		case now := <-sweep.C:
			s.expire(now)

		case clientID := <-s.unregister:
			s.clientsMu.Lock()
			delete(s.clients, clientID)
			s.clientsMu.Unlock()
			// Protocol trace logging
			logger.UDP("UNREGISTER", clientID, fmt.Sprintf("total_subscribers=%d", len(s.clients)))

		case notification := <-s.Broadcast:
			s.broadcastNotification(notification)

		case <-s.stop:
			logger.Info("UDP hub stopping...")
			return
		}
	}
}

// expire drops subscribers that haven't re-registered within the TTL (a
// client that quit without UNREGISTER, or whose network went away).
func (s *NotificationServer) expire(now time.Time) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()
	for id, sub := range s.clients {
		if now.Sub(sub.lastSeen) > s.ttl {
			delete(s.clients, id)
			logger.UDP("EXPIRE", id, fmt.Sprintf("silent for %v, total_subscribers=%d", s.ttl, len(s.clients)))
		}
	}
}

// listenForRegistrations handles incoming UDP messages (client registration)
func (s *NotificationServer) listenForRegistrations() {
	buffer := make([]byte, maxDatagram)

	for {
		select {
		case <-s.stop:
			return
		default:
			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				if isClosedErr(err) {
					return
				}
				// On Windows an ICMP "port unreachable" from a vanished
				// subscriber surfaces here as a read error; keep serving.
				logger.Errorf("udp read error: %v", err)
				continue
			}

			message := string(buffer[:n])
			logger.Debugf("UDP message from %s: %s", addr.String(), message)

			// Simple protocol: "REGISTER [token]" to register, "UNREGISTER" to unregister
			if message == "REGISTER" || strings.HasPrefix(message, "REGISTER ") {
				sub := subscriber{addr: addr}
				if token := strings.TrimSpace(strings.TrimPrefix(message, "REGISTER")); token != "" {
					userID, err := s.verifyToken(token)
					if err != nil {
						logger.Warnf("UDP register from %s rejected: %v", addr, err)
						s.sendTo(addr, []byte("ERROR invalid token"))
						continue
					}
					sub.userID = userID
				}
				s.register <- sub
				// Send confirmation
				s.sendTo(addr, []byte("REGISTERED"))
			} else if message == "UNREGISTER" {
				s.unregister <- addr.String()
				s.sendTo(addr, []byte("UNREGISTERED"))
			} else if strings.HasPrefix(message, "BROADCAST ") {
				// Handle external broadcast request
				payload := strings.TrimPrefix(message, "BROADCAST ")
				var notification Notification
				if err := json.Unmarshal([]byte(payload), &notification); err == nil {
					s.Broadcast <- notification
					logger.Infof("Received external broadcast request from %s", addr.String())
				} else {
					logger.Warnf("Invalid broadcast payload from %s: %v", addr.String(), err)
				}
			} else {
				logger.Warnf("unknown UDP command from %s: %s", addr.String(), message)
			}
		}
	}
}

func (s *NotificationServer) verifyToken(token string) (string, error) {
	if s.verify == nil {
		return "", errors.New("token registration is not enabled")
	}
	return s.verify(token)
}

// broadcastNotification sends a notification to every subscriber, or, when it
// lists UserIDs, only to subscribers registered as one of those users.
func (s *NotificationServer) broadcastNotification(notification Notification) {
	var targets map[string]bool
	if len(notification.UserIDs) > 0 {
		targets = make(map[string]bool, len(notification.UserIDs))
		for _, id := range notification.UserIDs {
			targets[id] = true
		}
		notification.UserIDs = nil // routing only: never reveal who else was notified
	}

	data, err := json.Marshal(notification)
	if err != nil {
		logger.Errorf("failed to marshal notification: %v", err)
		return
	}

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	sent := 0
	for clientID, sub := range s.clients {
		if targets != nil && !targets[sub.userID] {
			continue
		}
		if err := s.sendTo(sub.addr, data); err != nil {
			logger.Errorf("failed to send to %s: %v", clientID, err)
			continue
		}
		sent++
	}

	// Protocol trace logging
	scope := "all"
	if targets != nil {
		scope = fmt.Sprintf("%d_target_users", len(targets))
	}
	logger.UDP("BROADCAST", fmt.Sprintf("%d_clients(%s)", sent, scope), notification.Type+": "+notification.Message)
}

// sendTo sends data to a specific UDP address
func (s *NotificationServer) sendTo(addr *net.UDPAddr, data []byte) error {
	_, err := s.conn.WriteToUDP(data, addr)
	return err
}

// SubscriberCount returns the number of registered subscribers.
func (s *NotificationServer) SubscriberCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

// Stop stops the UDP server
func (s *NotificationServer) Stop() error {
	close(s.stop)
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// SendNotification sends a notification (convenience method)
func (s *NotificationServer) SendNotification(notification Notification) {
	select {
	case s.Broadcast <- notification:
	default:
		logger.Warn("UDP broadcast channel full, dropping notification")
	}
}

func isClosedErr(err error) bool {
	return errors.Is(err, net.ErrClosed)
}
