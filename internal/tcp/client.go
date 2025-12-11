package tcp

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"mangahub/pkg/logger"
)

type Client struct {
	Conn net.Conn
	addr string
}

func NewClient(host string, port int) *Client {
	return &Client{
		addr: fmt.Sprintf("%s:%d", host, port),
	}
}

func (c *Client) Connect() error {
	conn, err := net.DialTimeout("tcp", c.addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to TCP server: %w", err)
	}
	c.Conn = conn
	logger.Infof("TCP client connected to %s", c.addr)
	return nil
}

func (c *Client) SendProgressUpdate(userID, mangaID string, chapter int) error {
	if c.Conn == nil {
		return fmt.Errorf("TCP connection not established")
	}

	update := NewProgressUpdate(userID, mangaID, chapter)
	data, err := json.Marshal(update)
	if err != nil {
		return err
	}

	_, err = c.Conn.Write(append(data, '\n'))
	return err
}

func (c *Client) Close() error {
	if c.Conn != nil {
		return c.Conn.Close()
	}
	return nil
}
