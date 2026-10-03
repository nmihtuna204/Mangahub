package views

import (
	"testing"
	"time"
)

func TestChatHistoryGoesBeforeLiveMessages(t *testing.T) {
	m := NewChatModel()
	m.SetUser("u1", "reader1")
	m.SetRoom("general", "General Chat", "", "")

	m, _ = m.Update(ChatMessageReceivedMsg{RoomID: "general", Username: "bob", Content: "live", Type: "message", Timestamp: time.Now()})
	m, _ = m.Update(ChatHistoryLoadedMsg{RoomID: "general", Messages: []ChatMessage{
		{UserID: "u1", Username: "reader1", Content: "older mine", Type: "message"},
		{UserID: "u2", Username: "bob", Content: "older", Type: "message"},
	}})
	if m.MessageCount() != 3 || m.messages[0].Content != "older mine" || m.messages[2].Content != "live" {
		t.Errorf("messages = %+v, want history first", m.messages)
	}
	if !m.messages[0].IsOwn || m.messages[1].IsOwn {
		t.Error("IsOwn not set from the history's user IDs")
	}

	// History for another room (a late response after switching) is ignored
	m, _ = m.Update(ChatHistoryLoadedMsg{RoomID: "manga_x", Messages: []ChatMessage{{Content: "elsewhere"}}})
	if m.MessageCount() != 3 {
		t.Error("history for a different room was shown")
	}
}

// Regression: switching rooms kept showing the previous room's messages.
func TestSwitchingRoomsClearsMessages(t *testing.T) {
	m := NewChatModel()
	m.SetRoom("general", "General Chat", "", "")
	m.AddMessage(ChatMessage{Content: "hi"})

	m.SetRoom("general", "General Chat", "", "")
	if m.MessageCount() != 1 {
		t.Error("re-entering the same room cleared its messages")
	}
	m.SetRoom("manga_m1", "One Piece Discussion", "m1", "One Piece")
	if m.MessageCount() != 0 {
		t.Error("messages from the previous room are still shown")
	}
}
