package udp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// maxTargetsPerDatagram keeps a targeted BROADCAST request (36-byte user IDs
// plus JSON quoting) well under the maximum UDP payload
const maxTargetsPerDatagram = 1000

// ChapterReleaseBatches builds the notifications announcing a chapter to the
// given users, split so each fits in one datagram.
func ChapterReleaseBatches(mangaID, mangaTitle string, chapter int, userIDs []string) []Notification {
	var out []Notification
	for start := 0; start < len(userIDs); start += maxTargetsPerDatagram {
		end := start + maxTargetsPerDatagram
		if end > len(userIDs) {
			end = len(userIDs)
		}
		out = append(out, NewChapterReleaseNotification(mangaID, mangaTitle, chapter, userIDs[start:end]))
	}
	return out
}

// BroadcastRequest encodes a notification as a "BROADCAST <json>" request
// for the notification server.
func BroadcastRequest(n Notification) ([]byte, error) {
	payload, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}
	return append([]byte("BROADCAST "), payload...), nil
}

// Broadcaster sends broadcast requests to a UDP notification server from a
// process that isn't the API server (e.g. data-cli).
type Broadcaster struct {
	addr string
}

// NewBroadcaster creates a broadcaster for the server at addr ("host:port").
func NewBroadcaster(addr string) *Broadcaster {
	return &Broadcaster{addr: addr}
}

// Send asks the server to deliver one notification.
func (b *Broadcaster) Send(n Notification) error {
	datagram, err := BroadcastRequest(n)
	if err != nil {
		return err
	}
	conn, err := net.Dial("udp", b.addr)
	if err != nil {
		return fmt.Errorf("dial udp %s: %w", b.addr, err)
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, err = conn.Write(datagram)
	return err
}

// NotifyChapterRelease announces a chapter to the given users (a chapters.Notifier).
func (b *Broadcaster) NotifyChapterRelease(ctx context.Context, mangaID, mangaTitle string, chapter int, userIDs []string) error {
	for _, n := range ChapterReleaseBatches(mangaID, mangaTitle, chapter, userIDs) {
		if err := b.Send(n); err != nil {
			return err
		}
	}
	return nil
}
