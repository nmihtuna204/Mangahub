// Package tcp - TCP Synchronization Server Implementation
// Quản lý TCP connections và broadcast messages đến clients
// Chức năng:
//   - Accept nhiều TCP connections đồng thời
//   - Maintain danh sách active clients
//   - Broadcast progress updates đến tất cả clients
//   - Handle client disconnect gracefully
//   - JSON message protocol
//   - Concurrent goroutine cho mỗi client
package tcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"mangahub/pkg/logger"
)

type ClientID string

type client struct {
	id   ClientID
	conn net.Conn
	send chan []byte
}

type ProgressSyncServer struct {
	Addr       string
	listenerMu sync.Mutex // Serve sets listener while Stop may run on another goroutine
	listener   net.Listener
	clientsMu  sync.RWMutex
	clients    map[ClientID]*client
	Broadcast  chan ProgressUpdate
	register   chan *client
	unregister chan *client
	stop       chan struct{}
}

func NewProgressSyncServer(host string, port int) *ProgressSyncServer {
	return &ProgressSyncServer{
		Addr:       fmt.Sprintf("%s:%d", host, port),
		clients:    make(map[ClientID]*client),
		Broadcast:  make(chan ProgressUpdate, 100),
		register:   make(chan *client),
		unregister: make(chan *client),
		stop:       make(chan struct{}),
	}
}

func (s *ProgressSyncServer) Start() error {
	l, err := net.Listen("tcp", s.addr())
	if err != nil {
		return fmt.Errorf("listen tcp: %w", err)
	}
	return s.Serve(l)
}

// Serve accepts sync clients on an existing listener until Stop is called.
func (s *ProgressSyncServer) Serve(l net.Listener) error {
	s.listenerMu.Lock()
	s.listener = l
	s.listenerMu.Unlock()
	logger.Infof("TCP Progress Sync Server listening on %s", l.Addr())

	go s.runHub()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return nil
			default:
				logger.Errorf("accept error: %v", err)
				continue
			}
		}
		// Protocol trace logging
		logger.TCP("CONNECT", conn.RemoteAddr().String(), "", "new connection accepted")
		go s.handleConnection(conn)
	}
}

func (s *ProgressSyncServer) addr() string {
	return s.Addr
}

func (s *ProgressSyncServer) runHub() {
	for {
		select {
		case c := <-s.register:
			s.clientsMu.Lock()
			s.clients[c.id] = c
			s.clientsMu.Unlock()
			// Protocol trace logging
			logger.TCP("REGISTER", c.conn.RemoteAddr().String(), string(c.id), fmt.Sprintf("total_clients=%d", len(s.clients)))

		case c := <-s.unregister:
			s.clientsMu.Lock()
			if _, ok := s.clients[c.id]; ok {
				delete(s.clients, c.id)
				close(c.send)
				// Protocol trace logging
				logger.TCP("UNREGISTER", c.conn.RemoteAddr().String(), string(c.id), fmt.Sprintf("total_clients=%d", len(s.clients)))
			}
			s.clientsMu.Unlock()

		case update := <-s.Broadcast:
			data, err := json.Marshal(update)
			if err != nil {
				logger.Errorf("failed to marshal update: %v", err)
				continue
			}
			// The same slice goes to every client, so it must be complete
			// (newline included) and never modified after this point
			s.broadcastBytes(append(data, '\n'))

		case <-s.stop:
			logger.Info("TCP hub stopping...")
			return
		}
	}
}

func (s *ProgressSyncServer) broadcastBytes(data []byte) {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for _, c := range s.clients {
		select {
		case c.send <- data:
		default:
			logger.Warnf("client send buffer full, dropping message for client %s", c.id)
		}
	}
}

func (s *ProgressSyncServer) handleConnection(conn net.Conn) {
	id := ClientID(conn.RemoteAddr().String())
	c := &client{
		id:   id,
		conn: conn,
		send: make(chan []byte, 16),
	}

	s.register <- c

	// writeLoop exits when unregister closes c.send; on a write error it
	// closes the connection, which in turn ends readLoop below.
	go s.writeLoop(c)

	s.readLoop(c) // returns when the client disconnects
	s.unregister <- c
	_ = conn.Close()
}

func (s *ProgressSyncServer) readLoop(c *client) {
	reader := bufio.NewScanner(c.conn)
	for reader.Scan() {
		line := reader.Bytes()
		var update ProgressUpdate
		if err := json.Unmarshal(line, &update); err != nil {
			logger.Warnf("invalid JSON from %s: %v", c.id, err)
			continue
		}
		logger.Debugf("received progress from %s: %#v", c.id, update)

		s.Broadcast <- update
	}
	if err := reader.Err(); err != nil {
		logger.Warnf("read error from %s: %v", c.id, err)
	}
}

func (s *ProgressSyncServer) writeLoop(c *client) {
	for msg := range c.send {
		_, err := c.conn.Write(msg)
		if err != nil {
			logger.Warnf("write error to %s: %v", c.id, err)
			_ = c.conn.Close()
			// Drain until unregister closes the channel, so broadcasts in the
			// meantime don't log "buffer full" for a client that is going away.
			for range c.send {
			}
			return
		}
	}
}

// ClientCount returns the number of connected sync clients.
func (s *ProgressSyncServer) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

func (s *ProgressSyncServer) Stop() error {
	close(s.stop)
	s.listenerMu.Lock()
	defer s.listenerMu.Unlock()
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}
