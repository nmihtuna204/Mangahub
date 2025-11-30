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

	"github.com/yourusername/mangahub/pkg/logger"
)

type ClientID string

type client struct {
	id   ClientID
	conn net.Conn
	send chan []byte
}

type ProgressSyncServer struct {
	Addr        string
	listener    net.Listener
	clientsMu   sync.RWMutex
	clients     map[ClientID]*client
	Broadcast   chan ProgressUpdate
	register    chan *client
	unregister  chan *client
	stop        chan struct{}
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
	s.listener = l
	logger.Infof("TCP Progress Sync Server listening on %s", s.addr())

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
		logger.Infof("New TCP client connected from %s", conn.RemoteAddr())
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
			logger.Infof("Client registered: %s (total: %d)", c.id, len(s.clients))

		case c := <-s.unregister:
			s.clientsMu.Lock()
			if _, ok := s.clients[c.id]; ok {
				delete(s.clients, c.id)
				close(c.send)
				logger.Infof("Client unregistered: %s (total: %d)", c.id, len(s.clients))
			}
			s.clientsMu.Unlock()

		case update := <-s.Broadcast:
			data, err := json.Marshal(update)
			if err != nil {
				logger.Errorf("failed to marshal update: %v", err)
				continue
			}
			s.broadcastBytes(data)

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

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		s.readLoop(c)
	}()

	go func() {
		defer wg.Done()
		s.writeLoop(c)
	}()

	wg.Wait()
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
		_, err := c.conn.Write(append(msg, '\n'))
		if err != nil {
			logger.Warnf("write error to %s: %v", c.id, err)
			return
		}
	}
}

func (s *ProgressSyncServer) Stop() error {
	close(s.stop)
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

