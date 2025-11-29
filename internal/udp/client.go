package udp

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/yourusername/mangahub/pkg/logger"
)

// Client represents a UDP notification client
type Client struct {
	ServerAddr string
	conn       *net.UDPConn
	OnNotification func(Notification)
	stop       chan struct{}
}

// NewClient creates a new UDP client
func NewClient(serverHost string, serverPort int) *Client {
	return &Client{
		ServerAddr: fmt.Sprintf("%s:%d", serverHost, serverPort),
		stop:       make(chan struct{}),
	}
}

// Connect connects to the UDP server and registers
func (c *Client) Connect() error {
	serverAddr, err := net.ResolveUDPAddr("udp", c.ServerAddr)
	if err != nil {
		return fmt.Errorf("resolve server addr: %w", err)
	}

	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return fmt.Errorf("dial udp: %w", err)
	}
	c.conn = conn

	// Send registration message
	_, err = c.conn.Write([]byte("REGISTER"))
	if err != nil {
		return fmt.Errorf("send register: %w", err)
	}

	// Wait for confirmation
	buffer := make([]byte, 1024)
	c.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := c.conn.Read(buffer)
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}

	confirmation := string(buffer[:n])
	if confirmation != "REGISTERED" {
		return fmt.Errorf("unexpected confirmation: %s", confirmation)
	}

	logger.Infof("UDP client registered with server %s", c.ServerAddr)

	// Start listening for notifications
	go c.listen()

	return nil
}

// listen listens for incoming notifications
func (c *Client) listen() {
	buffer := make([]byte, 2048)
	c.conn.SetReadDeadline(time.Time{}) // Remove deadline

	for {
		select {
		case <-c.stop:
			return
		default:
			n, err := c.conn.Read(buffer)
			if err != nil {
				if !isClosedErr(err) {
					logger.Errorf("udp client read error: %v", err)
				}
				return
			}

			var notification Notification
			if err := json.Unmarshal(buffer[:n], &notification); err != nil {
				logger.Warnf("failed to unmarshal notification: %v", err)
				continue
			}

			if c.OnNotification != nil {
				c.OnNotification(notification)
			}
		}
	}
}

// Close closes the UDP client connection
func (c *Client) Close() error {
	close(c.stop)
	if c.conn != nil {
		// Send unregister message
		_, _ = c.conn.Write([]byte("UNREGISTER"))
		return c.conn.Close()
	}
	return nil
}
