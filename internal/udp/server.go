// Package udp - UDP Notification Server Implementation
// Quản lý UDP datagram communication cho push notifications
// Chức năng:
//   - Nhận REGISTER/UNREGISTER messages từ clients
//   - Maintain subscriber list
//   - Broadcast chapter notifications đến tất cả subscribers
//   - Connectionless protocol - không maintain state
//   - JSON datagram format
//   - Non-blocking sends
package udp

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/yourusername/mangahub/pkg/logger"
)

// NotificationServer manages UDP notification broadcasting
type NotificationServer struct {
	Addr       string
	conn       *net.UDPConn
	clientsMu  sync.RWMutex
	clients    map[string]*net.UDPAddr // clientID -> address
	Broadcast  chan Notification
	register   chan *net.UDPAddr
	unregister chan string
	stop       chan struct{}
}

// NewNotificationServer creates a new UDP notification server
func NewNotificationServer(host string, port int) *NotificationServer {
	return &NotificationServer{
		Addr:       fmt.Sprintf("%s:%d", host, port),
		clients:    make(map[string]*net.UDPAddr),
		Broadcast:  make(chan Notification, 100),
		register:   make(chan *net.UDPAddr),
		unregister: make(chan string),
		stop:       make(chan struct{}),
	}
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
	s.conn = conn

	logger.Infof("UDP Notification Server listening on %s", s.Addr)

	go s.runHub()
	go s.listenForRegistrations()

	<-s.stop
	return nil
}

// runHub manages client registration and broadcasting
func (s *NotificationServer) runHub() {
	for {
		select {
		case addr := <-s.register:
			clientID := addr.String()
			s.clientsMu.Lock()
			s.clients[clientID] = addr
			s.clientsMu.Unlock()
			logger.Infof("UDP client registered: %s (total: %d)", clientID, len(s.clients))

		case clientID := <-s.unregister:
			s.clientsMu.Lock()
			delete(s.clients, clientID)
			s.clientsMu.Unlock()
			logger.Infof("UDP client unregistered: %s (total: %d)", clientID, len(s.clients))

		case notification := <-s.Broadcast:
			s.broadcastNotification(notification)

		case <-s.stop:
			logger.Info("UDP hub stopping...")
			return
		}
	}
}

// listenForRegistrations handles incoming UDP messages (client registration)
func (s *NotificationServer) listenForRegistrations() {
	buffer := make([]byte, 2048)
	
	for {
		select {
		case <-s.stop:
			return
		default:
			n, addr, err := s.conn.ReadFromUDP(buffer)
			if err != nil {
				if !isClosedErr(err) {
					logger.Errorf("udp read error: %v", err)
				}
				continue
			}

			message := string(buffer[:n])
			logger.Debugf("UDP message from %s: %s", addr.String(), message)

			// Simple protocol: "REGISTER" to register, "UNREGISTER" to unregister
			switch message {
			case "REGISTER":
				s.register <- addr
				// Send confirmation
				s.sendTo(addr, []byte("REGISTERED"))
			case "UNREGISTER":
				s.unregister <- addr.String()
				s.sendTo(addr, []byte("UNREGISTERED"))
			default:
				logger.Warnf("unknown UDP command from %s: %s", addr.String(), message)
			}
		}
	}
}

// broadcastNotification sends notification to all registered clients
func (s *NotificationServer) broadcastNotification(notification Notification) {
	data, err := json.Marshal(notification)
	if err != nil {
		logger.Errorf("failed to marshal notification: %v", err)
		return
	}

	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	if len(s.clients) == 0 {
		logger.Debug("no UDP clients to broadcast to")
		return
	}

	logger.Infof("Broadcasting notification to %d UDP clients: %s", len(s.clients), notification.Message)

	for clientID, addr := range s.clients {
		if err := s.sendTo(addr, data); err != nil {
			logger.Errorf("failed to send to %s: %v", clientID, err)
		}
	}
}

// sendTo sends data to a specific UDP address
func (s *NotificationServer) sendTo(addr *net.UDPAddr, data []byte) error {
	_, err := s.conn.WriteToUDP(data, addr)
	return err
}

// Stop stops the UDP server
func (s *NotificationServer) Stop() error {
	close(s.stop)
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
	return err != nil && err.Error() == "use of closed network connection"
}
