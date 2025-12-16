// Package views - Chat View Component
// Real-time chat interface with message history and input
// Integrates with WebSocket for live messaging
package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =====================================
// CHAT MESSAGE MODEL
// =====================================

// ChatMessage represents a single chat message for display
type ChatMessage struct {
	ID        string
	RoomID    string
	UserID    string
	Username  string
	Content   string
	Type      string // text, join, leave, system
	Timestamp time.Time
	IsOwn     bool // true if sent by current user
}

// =====================================
// STYLES
// =====================================

var (
	// Header styles
	chatHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D4FF")).
			Background(lipgloss.Color("#1a1a2e")).
			Padding(0, 1).
			Width(100)

	connectionOnlineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF88")).
				Bold(true)

	connectionOfflineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF4444")).
				Bold(true)

	connectionReconnectStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("#FFAA00")).
					Bold(true)

	// Message styles
	usernameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00D4FF"))

	ownUsernameStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#00FF88"))

	timestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)

	messageContentStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))

	systemMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888")).
				Italic(true).
				Align(lipgloss.Center)

	joinMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00FF88")).
				Italic(true)

	leaveMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF8800")).
				Italic(true)

	// Input area styles
	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#00D4FF")).
				Padding(0, 1)

	inputBorderDisabledStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("#FF4444")).
					Padding(0, 1)

	inputHintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)

	// Room info
	roomInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA"))

	userCountStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00D4FF")).
			Bold(true)
)

// =====================================
// CHAT MODEL
// =====================================

// ConnectionStatus represents WebSocket connection state
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnecting
	StatusConnected
	StatusReconnecting
)

// ChatModel is the Bubble Tea model for chat view
type ChatModel struct {
	messages  []ChatMessage
	viewport  viewport.Model
	textarea  textarea.Model
	roomID    string
	roomName  string
	mangaID   string
	mangaName string
	userID    string
	username  string
	userCount int
	status    ConnectionStatus
	width     int
	height    int
	focused   bool
	ready     bool
}

// NewChatModel creates a new chat model
func NewChatModel() ChatModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.Prompt = "│ "
	ta.CharLimit = 500
	ta.SetWidth(80)
	ta.SetHeight(2)
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.SetContent("")

	return ChatModel{
		messages:  make([]ChatMessage, 0),
		viewport:  vp,
		textarea:  ta,
		status:    StatusDisconnected,
		focused:   true,
		userCount: 0,
	}
}

// =====================================
// TEA.MODEL INTERFACE
// =====================================

// Init implements tea.Model
func (m ChatModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update implements tea.Model
func (m ChatModel) Update(msg tea.Msg) (ChatModel, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateDimensions()

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.status == StatusConnected && strings.TrimSpace(m.textarea.Value()) != "" {
				// Return command to send message
				content := strings.TrimSpace(m.textarea.Value())
				m.textarea.Reset()
				return m, func() tea.Msg {
					return SendChatMsg{
						RoomID:  m.roomID,
						Content: content,
					}
				}
			}

		case "esc":
			m.textarea.Blur()
			m.focused = false
			return m, nil

		case "tab":
			if !m.focused {
				m.textarea.Focus()
				m.focused = true
				return m, textarea.Blink
			}
		}

	case ChatMessageReceivedMsg:
		// Add message to history
		m.messages = append(m.messages, ChatMessage{
			ID:        msg.ID,
			RoomID:    msg.RoomID,
			UserID:    msg.UserID,
			Username:  msg.Username,
			Content:   msg.Content,
			Type:      msg.Type,
			Timestamp: msg.Timestamp,
			IsOwn:     msg.UserID == m.userID,
		})
		m.updateViewportContent()
		// Scroll to bottom
		m.viewport.GotoBottom()

	case ChatRoomJoinedMsg:
		m.roomID = msg.RoomID
		m.roomName = msg.RoomName
		m.mangaID = msg.MangaID
		m.mangaName = msg.MangaName
		m.userCount = msg.UserCount
		m.status = StatusConnected
		// Clear old messages
		m.messages = make([]ChatMessage, 0)
		m.updateViewportContent()

	case ChatConnectionStatusMsg:
		m.status = msg.Status
		m.userCount = msg.UserCount

	case ChatUserCountMsg:
		m.userCount = msg.Count
	}

	// Update textarea if focused
	if m.focused && m.status == StatusConnected {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m ChatModel) View() string {
	if m.width == 0 {
		return "Loading chat..."
	}

	var b strings.Builder

	// Header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Messages viewport
	b.WriteString(m.renderMessages())
	b.WriteString("\n")

	// Input area
	b.WriteString(m.renderInput())

	return b.String()
}

// =====================================
// RENDERING HELPERS
// =====================================

func (m ChatModel) renderHeader() string {
	// Connection status indicator
	var statusIndicator string
	switch m.status {
	case StatusConnected:
		statusIndicator = connectionOnlineStyle.Render("● Connected")
	case StatusConnecting:
		statusIndicator = connectionReconnectStyle.Render("◐ Connecting...")
	case StatusReconnecting:
		statusIndicator = connectionReconnectStyle.Render("↻ Reconnecting...")
	default:
		statusIndicator = connectionOfflineStyle.Render("○ Disconnected")
	}

	// Room name
	roomDisplay := m.roomName
	if roomDisplay == "" {
		roomDisplay = m.roomID
	}
	if m.mangaName != "" {
		roomDisplay = fmt.Sprintf("%s Discussion", m.mangaName)
	}

	// User count
	userInfo := ""
	if m.userCount > 0 {
		userInfo = userCountStyle.Render(fmt.Sprintf(" [%d online]", m.userCount))
	}

	header := fmt.Sprintf("💬 %s%s  %s",
		roomDisplay,
		userInfo,
		statusIndicator,
	)

	return chatHeaderStyle.Width(m.width).Render(header)
}

func (m ChatModel) renderMessages() string {
	// Create viewport border
	viewportStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#333333")).
		Width(m.width - 2).
		Height(m.height - 8) // Reserve space for header and input

	return viewportStyle.Render(m.viewport.View())
}

func (m ChatModel) renderInput() string {
	// Choose style based on connection status
	var borderStyle lipgloss.Style
	if m.status == StatusConnected {
		borderStyle = inputBorderStyle
	} else {
		borderStyle = inputBorderDisabledStyle
	}

	// Input container
	inputWidth := m.width - 4
	m.textarea.SetWidth(inputWidth - 4)

	input := borderStyle.Width(inputWidth).Render(m.textarea.View())

	// Hint text
	var hint string
	if m.status != StatusConnected {
		hint = inputHintStyle.Render("  ⚠ Connection required to send messages")
	} else if m.focused {
		hint = inputHintStyle.Render("  Enter: Send • Esc: Unfocus • Tab: Focus input")
	} else {
		hint = inputHintStyle.Render("  Tab: Focus input • Esc: Back")
	}

	return input + "\n" + hint
}

func (m *ChatModel) updateDimensions() {
	// Header takes ~2 lines, input takes ~4 lines
	viewportHeight := m.height - 8
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	m.viewport.Width = m.width - 4
	m.viewport.Height = viewportHeight
	m.textarea.SetWidth(m.width - 8)

	m.ready = true
	m.updateViewportContent()
}

func (m *ChatModel) updateViewportContent() {
	if !m.ready {
		return
	}

	var lines []string

	if len(m.messages) == 0 {
		// Empty state
		emptyMsg := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true).
			Align(lipgloss.Center).
			Width(m.viewport.Width).
			Render("\n\n  No messages yet. Start the conversation! 💬\n")
		lines = append(lines, emptyMsg)
	} else {
		for _, msg := range m.messages {
			lines = append(lines, m.formatMessage(msg))
		}
	}

	content := strings.Join(lines, "\n")
	m.viewport.SetContent(content)
}

func (m ChatModel) formatMessage(msg ChatMessage) string {
	timestamp := timestampStyle.Render(formatChatTime(msg.Timestamp))

	switch msg.Type {
	case "join":
		return joinMessageStyle.Render(fmt.Sprintf("  → %s joined the room  %s", msg.Username, timestamp))

	case "leave":
		return leaveMessageStyle.Render(fmt.Sprintf("  ← %s left the room  %s", msg.Username, timestamp))

	case "system":
		return systemMessageStyle.Width(m.viewport.Width).Render(msg.Content)

	default: // "text" or empty
		var usernameRender string
		if msg.IsOwn {
			usernameRender = ownUsernameStyle.Render(msg.Username)
		} else {
			usernameRender = usernameStyle.Render(msg.Username)
		}

		content := messageContentStyle.Render(msg.Content)

		return fmt.Sprintf("  %s %s: %s", timestamp, usernameRender, content)
	}
}

func formatChatTime(t time.Time) string {
	now := time.Now()
	if t.Day() == now.Day() && t.Month() == now.Month() && t.Year() == now.Year() {
		return t.Format("[15:04]")
	}
	return t.Format("[Jan 2 15:04]")
}

// =====================================
// PUBLIC METHODS
// =====================================

// SetUser sets the current user info
func (m *ChatModel) SetUser(userID, username string) {
	m.userID = userID
	m.username = username
}

// SetRoom sets the current room info
func (m *ChatModel) SetRoom(roomID, roomName, mangaID, mangaName string) {
	m.roomID = roomID
	m.roomName = roomName
	m.mangaID = mangaID
	m.mangaName = mangaName
}

// SetStatus sets the connection status
func (m *ChatModel) SetStatus(status ConnectionStatus) {
	m.status = status
}

// IsInputFocused reports whether the chat textarea currently has focus.
func (m ChatModel) IsInputFocused() bool {
	return m.focused && m.textarea.Focused()
}

// ClearMessages clears all messages
func (m *ChatModel) ClearMessages() {
	m.messages = make([]ChatMessage, 0)
	m.updateViewportContent()
}

// AddMessage adds a message to the chat
func (m *ChatModel) AddMessage(msg ChatMessage) {
	m.messages = append(m.messages, msg)
	m.updateViewportContent()
	m.viewport.GotoBottom()
}

// MessageCount returns the number of messages
func (m ChatModel) MessageCount() int {
	return len(m.messages)
}

// RoomID returns the current room ID
func (m ChatModel) RoomID() string {
	return m.roomID
}

// =====================================
// BUBBLE TEA MESSAGES
// =====================================

// SendChatMsg is returned when user wants to send a message
type SendChatMsg struct {
	RoomID  string
	Content string
}

// ChatMessageReceivedMsg is sent when a message is received
type ChatMessageReceivedMsg struct {
	ID        string
	RoomID    string
	UserID    string
	Username  string
	Content   string
	Type      string
	Timestamp time.Time
}

// ChatRoomJoinedMsg is sent when successfully joined a room
type ChatRoomJoinedMsg struct {
	RoomID    string
	RoomName  string
	MangaID   string
	MangaName string
	UserCount int
}

// ChatConnectionStatusMsg is sent when connection status changes
type ChatConnectionStatusMsg struct {
	Status    ConnectionStatus
	UserCount int
}

// ChatUserCountMsg is sent when user count changes
type ChatUserCountMsg struct {
	Count int
}
