// Package tui - Root Application Model
// Main Bubble Tea application cho MangaHub TUI
// Quản lý views, navigation, và global state
//
// Architecture:
//   - Root Model chứa tất cả views
//   - Active view được render trong main area
//   - Persistent footer hiển thị global keybindings
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/network"
	"mangahub/internal/tui/styles"
	"mangahub/internal/tui/views"
	"mangahub/pkg/models"
)

// =====================================
// VIEW ENUM - Available Views
// =====================================

// View represents the current active view
type View int

const (
	ViewDashboard View = iota
	ViewSearch
	ViewBrowse
	ViewLibrary
	ViewDetail
	ViewProfile
	ViewActivity
	ViewStats
	ViewSettings
	ViewAuth
	ViewHelp
	ViewChat
)

// =====================================
// MESSAGES - Inter-view Communication
// =====================================

// AppReadyMsg signals app initialization complete
type AppReadyMsg struct{}

// ViewChangeMsg requests a view change
type ViewChangeMsg struct {
	View    View
	Payload interface{} // Optional data to pass to new view
}

// ErrorMsg contains an error to display
type ErrorMsg struct {
	Error error
}

// UserLoggedInMsg signals successful login
type UserLoggedInMsg struct {
	User *models.User
}

// MangaSelectedMsg signals a manga was selected
type MangaSelectedMsg struct {
	MangaID string
	Manga   *models.Manga
}

// RefreshMsg signals data refresh needed
type RefreshMsg struct{}

// WindowSizeMsg carries terminal dimensions
type WindowSizeMsg struct {
	Width  int
	Height int
}

// =====================================
// ROOT MODEL - Main Application State
// =====================================

// Model is the root application model
type Model struct {
	// Window size
	width  int
	height int

	// Current view
	currentView  View
	previousView View

	// User state
	user          *models.User
	authenticated bool

	// API client
	client *api.Client

	// Key bindings
	keys KeyMap

	// Theme
	theme *styles.Theme

	// UI Components
	spinner spinner.Model

	// View models (properly typed)
	dashboardModel views.DashboardModel
	searchModel    views.SearchModel
	libraryModel   views.LibraryModel
	browseModel    views.BrowseModel
	detailModel    views.DetailModel
	activityModel  views.ActivityModel
	authModel      views.AuthModel
	helpModel      views.HelpModel

	// Command palette
	paletteModel views.PaletteModel

	// Chat view
	chatModel views.ChatModel

	// Rating modal and comments view
	ratingModal  views.RatingModal
	commentsView views.CommentsView
	showRating   bool
	showComments bool

	// WebSocket client for real-time chat
	wsClient *network.WSClient

	// UDP listener for real-time notifications
	udpListener *network.UDPListener

	// Notification state
	unreadChatCount int
	toast           *ToastModel

	// Input mode tracking
	inputMode bool // true when typing in forms (disables global shortcuts)

	// Error handling
	lastError error

	// Loading state
	loading bool

	// Selected manga (for detail view)
	selectedMangaID string
}

// NewApp creates a new root model (exported for cmd/tui)
func NewApp() Model {
	return New()
}

// New creates a new root model
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	return Model{
		currentView:    ViewDashboard,
		previousView:   ViewDashboard,
		keys:           DefaultKeyMap(),
		theme:          styles.DefaultTheme,
		spinner:        s,
		client:         api.GetClient(),
		authenticated:  api.GetClient().IsAuthenticated(),
		dashboardModel: views.NewDashboard(),
		searchModel:    views.NewSearch(),
		libraryModel:   views.NewLibrary(),
		browseModel:    views.NewBrowse(),
		activityModel:  views.NewActivity(),
		authModel:      views.NewAuth(),
		helpModel:      views.NewHelp(),
		paletteModel:   views.NewPalette(),
		chatModel:      views.NewChatModel(),
		wsClient:       network.NewWSClient(),
		udpListener:    network.NewUDPListener(),
		toast:          NewToast(),
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.checkAuth,
		m.dashboardModel.Init(),
	)
}

// checkAuth verifies authentication status on startup
func (m Model) checkAuth() tea.Msg {
	if m.client.IsAuthenticated() {
		ctx := context.Background()
		user, err := m.client.GetCurrentUser(ctx)
		if err != nil {
			// Token expired or invalid
			m.client.ClearToken()
			return ViewChangeMsg{View: ViewAuth}
		}
		return UserLoggedInMsg{User: user}
	}
	return ViewChangeMsg{View: ViewDashboard}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Update all view dimensions
		m.dashboardModel.SetWidth(msg.Width - 4)
		m.dashboardModel.SetHeight(msg.Height - 6)
		// Update chat dimensions
		m.chatModel, _ = m.chatModel.Update(msg)
		m.searchModel.SetWidth(msg.Width - 4)
		m.searchModel.SetHeight(msg.Height - 6)
		m.libraryModel.SetWidth(msg.Width - 4)
		m.libraryModel.SetHeight(msg.Height - 6)
		m.browseModel.SetWidth(msg.Width - 4)
		m.browseModel.SetHeight(msg.Height - 6)
		m.activityModel.SetWidth(msg.Width - 4)
		m.activityModel.SetHeight(msg.Height - 6)
		m.authModel.SetWidth(msg.Width - 4)
		m.authModel.SetHeight(msg.Height - 6)
		m.helpModel.SetWidth(msg.Width - 4)
		m.helpModel.SetHeight(msg.Height - 6)
		m.paletteModel.SetWidth(msg.Width)
		m.paletteModel.SetHeight(msg.Height)
		// Update modal and overlay dimensions
		if m.showRating {
			m.ratingModal, _ = m.ratingModal.Update(msg)
		}
		if m.showComments {
			m.commentsView, _ = m.commentsView.Update(msg)
		}
		return m, nil

	case tea.KeyMsg:
		// Check if rating modal is open - handle it first
		if m.showRating {
			var cmd tea.Cmd
			m.ratingModal, cmd = m.ratingModal.Update(msg)
			return m, cmd
		}

		// Check if comments view is open - handle it first
		if m.showComments {
			var cmd tea.Cmd
			m.commentsView, cmd = m.commentsView.Update(msg)
			return m, cmd
		}

		// Check if palette is open - if so, handle it first
		if m.paletteModel.IsVisible() {
			var cmd tea.Cmd
			m.paletteModel, cmd = m.paletteModel.Update(msg)
			return m, cmd
		}

		// Always handle these keys regardless of input mode
		switch msg.String() {
		case "ctrl+p":
			// Open command palette
			m.paletteModel.Show()
			return m, m.paletteModel.Init()

		case "ctrl+c":
			// Force quit
			return m, tea.Quit

		case "?":
			// Open help
			if m.currentView != ViewHelp {
				m.previousView = m.currentView
				m.currentView = ViewHelp
				return m, m.helpModel.Init()
			}
			return m, nil

		case "esc":
			// Check if rating modal or comments view is open
			if m.showRating {
				m.showRating = false
				return m, nil
			}
			if m.showComments {
				m.showComments = false
				return m, nil
			}
			// Always allow ESC to go back
			if m.currentView != ViewDashboard {
				m.currentView = m.previousView
				if m.currentView == m.previousView {
					m.currentView = ViewDashboard
				}
			}
			return m, nil
		}

		// Detect input mode based on focused inputs in the active view
		m.inputMode = m.isInputFocused()

		// If in input mode, pass to view immediately (don't check global shortcuts)
		if m.inputMode {
			return m.updateCurrentView(msg)
		}

		// Global key handling (only when NOT in input mode)
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Dashboard):
			if m.currentView != ViewDashboard {
				m.previousView = m.currentView
				m.currentView = ViewDashboard
				return m, m.dashboardModel.Init()
			}
			return m, nil

		case key.Matches(msg, m.keys.Search):
			if m.currentView != ViewSearch {
				m.previousView = m.currentView
				m.currentView = ViewSearch
				return m, m.searchModel.Focus()
			}
			return m, nil

		case key.Matches(msg, m.keys.Library):
			if !m.authenticated {
				m.previousView = m.currentView
				m.currentView = ViewAuth
				return m, m.authModel.Init()
			}
			if m.currentView != ViewLibrary {
				m.previousView = m.currentView
				m.currentView = ViewLibrary
				return m, m.libraryModel.Init()
			}
			return m, nil

		case key.Matches(msg, m.keys.Browse):
			if m.currentView != ViewBrowse {
				m.previousView = m.currentView
				m.currentView = ViewBrowse
				return m, m.browseModel.Init()
			}
			return m, nil

		case key.Matches(msg, m.keys.Activity):
			if m.currentView != ViewActivity {
				m.previousView = m.currentView
				m.currentView = ViewActivity
				return m, m.activityModel.Init()
			}
			return m, nil

		case key.Matches(msg, m.keys.Login):
			if m.authenticated {
				// Already logged in, logout instead
				m.client.ClearToken()
				m.authenticated = false
				m.user = nil
				// Stop UDP listener on logout
				m.udpListener.Stop()
				return m, nil
			}
			if m.currentView != ViewAuth {
				m.previousView = m.currentView
				m.currentView = ViewAuth
				return m, m.authModel.Init()
			}
			return m, nil

		case key.Matches(msg, m.keys.Chat):
			// Go to chat view
			if !m.authenticated {
				m.previousView = m.currentView
				m.currentView = ViewAuth
				return m, m.authModel.Init()
			}
			if m.currentView != ViewChat {
				m.previousView = m.currentView
				m.currentView = ViewChat
				// Connect to general chat if no room specified
				if m.chatModel.RoomID() == "" {
					m.chatModel.SetRoom("general", "General Chat", "", "")
				}
				wsURL := strings.Replace(m.client.GetBaseURL(), "http://", "ws://", 1)
				wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
				return m, tea.Batch(
					m.chatModel.Init(),
					m.wsClient.Connect(wsURL, m.client.GetToken(), m.chatModel.RoomID()),
				)
			}
			return m, nil

		default:
			// Pass to current view
			return m.updateCurrentView(msg)
		}

	case views.CommandSelectedMsg:
		// Handle command from palette
		return m.handleCommand(msg.CommandID)

	case views.PaletteCloseMsg:
		m.paletteModel.Hide()
		return m, nil

	case ViewChangeMsg:
		m.previousView = m.currentView
		m.currentView = msg.View
		if mangaMsg, ok := msg.Payload.(MangaSelectedMsg); ok {
			m.selectedMangaID = mangaMsg.MangaID
			m.detailModel = views.NewDetail(mangaMsg.MangaID)
			return m, m.detailModel.Init()
		}
		return m, nil

	case UserLoggedInMsg:
		m.user = msg.User
		m.authenticated = true
		// Update chat user info
		m.chatModel.SetUser(msg.User.ID, msg.User.Username)
		// Start UDP listener for real-time notifications
		return m, m.udpListener.Start("9091")

	case ErrorMsg:
		m.lastError = msg.Error
		return m, nil

	// =====================================
	// CHAT & WEBSOCKET MESSAGES
	// =====================================

	case views.ShowRatingMsg:
		// Show rating modal
		if !m.authenticated {
			m.toast.Show("Please login to rate manga", 3*time.Second)
			return m, nil
		}
		m.ratingModal = views.NewRatingModal(msg.MangaID, msg.MangaTitle)
		m.showRating = true
		return m, m.ratingModal.Init()

	case views.ShowCommentsMsg:
		// Show comments view
		m.commentsView = views.NewCommentsView(msg.MangaID, msg.MangaTitle)
		m.showComments = true
		return m, m.commentsView.Init()

	case views.RatingSubmittedMsg:
		// Rating was submitted successfully
		m.showRating = false
		m.toast.Show("Rating submitted successfully!", 3*time.Second)
		// Reload detail view to show updated rating
		return m, m.detailModel.Init()

	case views.RatingErrorMsg:
		// Rating submission failed
		m.toast.Show(fmt.Sprintf("Failed to submit rating: %v", msg.Error), 5*time.Second)
		return m, nil

	case network.JoinRoomMsg:
		// User requested to join a chat room
		if !m.authenticated {
			m.previousView = m.currentView
			m.currentView = ViewAuth
			return m, m.authModel.Init()
		}
		// Set room info on chat model
		m.chatModel.SetRoom(msg.RoomID, msg.RoomName, msg.MangaID, msg.MangaName)
		m.previousView = m.currentView
		m.currentView = ViewChat
		// Connect WebSocket
		wsURL := strings.Replace(m.client.GetBaseURL(), "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		return m, tea.Batch(
			m.chatModel.Init(),
			m.wsClient.Connect(wsURL, m.client.GetToken(), msg.RoomID),
		)

	case network.WSConnectedMsg:
		// WebSocket connected successfully
		m.chatModel.SetStatus(views.StatusConnected)
		// Mark unread as read when viewing chat
		if m.currentView == ViewChat {
			m.unreadChatCount = 0
		}
		// Start listening for messages
		return m, m.wsClient.ListenForMessages()

	case network.WSDisconnectedMsg:
		// WebSocket disconnected
		m.chatModel.SetStatus(views.StatusDisconnected)
		// If we're in chat view, try to reconnect
		if m.currentView == ViewChat {
			return m, m.wsClient.Reconnect()
		}
		return m, nil

	case network.WSReconnectingMsg:
		m.chatModel.SetStatus(views.StatusReconnecting)
		return m, m.wsClient.Reconnect()

	case network.WSErrorMsg:
		m.lastError = msg.Err
		m.chatModel.SetStatus(views.StatusDisconnected)
		if m.currentView == ViewChat {
			return m, m.wsClient.Reconnect()
		}
		return m, nil

	case network.ChatMessageMsg:
		// Incoming chat message from WebSocket
		chatMsg := views.ChatMessageReceivedMsg{
			ID:        msg.ID,
			RoomID:    msg.RoomID,
			UserID:    msg.UserID,
			Username:  msg.Username,
			Content:   msg.Content,
			Type:      msg.Type,
			Timestamp: msg.Timestamp,
		}
		// Update chat model
		m.chatModel, _ = m.chatModel.Update(chatMsg)
		// If not on chat view, increment unread count
		if m.currentView != ViewChat {
			m.unreadChatCount++
		}
		// Continue listening for messages
		return m, m.wsClient.ListenForMessages()

	case views.SendChatMsg:
		// User wants to send a chat message
		return m, m.wsClient.SendMessage(msg.RoomID, msg.Content)

	// =====================================
	// UDP NOTIFICATION MESSAGES
	// =====================================

	case network.UDPConnectedMsg:
		// UDP listener connected - start receiving notifications
		return m, m.udpListener.WaitForPacket()

	case network.UDPDisconnectedMsg:
		// UDP listener disconnected
		// Could add reconnection logic here if needed
		return m, nil

	case network.UDPErrorMsg:
		// UDP error occurred
		m.lastError = msg.Err
		return m, nil

	case network.UDPNotificationMsg:
		// Incoming UDP notification - show as toast
		notification := network.FormatNotification(msg)
		m.toast.Show(notification, 5*time.Second)
		// Continue listening for more notifications
		return m, m.udpListener.WaitForPacket()

	case ToastTickMsg:
		// Update toast timer
		if m.toast != nil {
			m.toast.Update(msg)
		}
		return m, nil

	case ToastShowMsg:
		// Show toast notification
		if m.toast != nil {
			m.toast.Show(msg.Content, msg.Duration)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	default:
		// Pass non-keyboard messages to current view
		return m.updateCurrentView(msg)
	}

	return m, tea.Batch(cmds...)
}

// updateCurrentView passes messages to the active view
func (m Model) updateCurrentView(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.currentView {
	case ViewDashboard:
		m.dashboardModel, cmd = m.dashboardModel.Update(msg)
	case ViewSearch:
		m.searchModel, cmd = m.searchModel.Update(msg)
		// Check for manga selection
		if selected := m.searchModel.GetSelectedManga(); selected != nil {
			if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
				m.selectedMangaID = selected.ID
				m.detailModel = views.NewDetail(selected.ID)
				m.previousView = m.currentView
				m.currentView = ViewDetail
				return m, m.detailModel.Init()
			}
		}
	case ViewLibrary:
		m.libraryModel, cmd = m.libraryModel.Update(msg)
		// Check for manga selection
		if selected := m.libraryModel.GetSelectedEntry(); selected != nil {
			if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
				m.selectedMangaID = selected.MangaID
				m.detailModel = views.NewDetail(selected.MangaID)
				m.previousView = m.currentView
				m.currentView = ViewDetail
				return m, m.detailModel.Init()
			}
		}
	case ViewBrowse:
		m.browseModel, cmd = m.browseModel.Update(msg)
		// Check for manga selection
		if selected := m.browseModel.GetSelectedManga(); selected != nil {
			if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "enter" {
				m.selectedMangaID = selected.ID
				m.detailModel = views.NewDetail(selected.ID)
				m.previousView = m.currentView
				m.currentView = ViewDetail
				return m, m.detailModel.Init()
			}
		}
	case ViewDetail:
		m.detailModel, cmd = m.detailModel.Update(msg)
	case ViewActivity:
		m.activityModel, cmd = m.activityModel.Update(msg)
	case ViewAuth:
		m.authModel, cmd = m.authModel.Update(msg)
		// Check for successful login
		if m.authModel.IsLoggedIn() {
			user := m.authModel.GetUser()
			if user != nil {
				m.user = user
				m.authenticated = true
				// Return to previous view or dashboard
				if m.previousView != ViewAuth {
					m.currentView = m.previousView
				} else {
					m.currentView = ViewDashboard
				}
				return m, m.dashboardModel.Init()
			}
		}
	case ViewHelp:
		m.helpModel, cmd = m.helpModel.Update(msg)
	case ViewChat:
		m.chatModel, cmd = m.chatModel.Update(msg)
		// Clear unread count when viewing chat
		m.unreadChatCount = 0
	}

	return m, cmd
}

// handleCommand processes commands from the command palette
func (m Model) handleCommand(commandID string) (tea.Model, tea.Cmd) {
	switch commandID {
	case "goto_dashboard":
		m.previousView = m.currentView
		m.currentView = ViewDashboard
		return m, m.dashboardModel.Init()
	case "goto_search":
		m.previousView = m.currentView
		m.currentView = ViewSearch
		return m, m.searchModel.Focus()
	case "goto_browse":
		m.previousView = m.currentView
		m.currentView = ViewBrowse
		return m, m.browseModel.Init()
	case "goto_library":
		if !m.authenticated {
			m.previousView = m.currentView
			m.currentView = ViewAuth
			return m, m.authModel.Init()
		}
		m.previousView = m.currentView
		m.currentView = ViewLibrary
		return m, m.libraryModel.Init()
	case "goto_activity":
		m.previousView = m.currentView
		m.currentView = ViewActivity
		return m, m.activityModel.Init()
	case "login":
		if m.authenticated {
			m.client.ClearToken()
			m.authenticated = false
			m.user = nil
			// Stop UDP listener on logout
			m.udpListener.Stop()
		} else {
			m.previousView = m.currentView
			m.currentView = ViewAuth
			return m, m.authModel.Init()
		}
	case "help":
		m.previousView = m.currentView
		m.currentView = ViewHelp
		return m, m.helpModel.Init()
	case "goto_chat":
		if !m.authenticated {
			m.previousView = m.currentView
			m.currentView = ViewAuth
			return m, m.authModel.Init()
		}
		m.previousView = m.currentView
		m.currentView = ViewChat
		// Connect to general chat if no room specified
		if m.chatModel.RoomID() == "" {
			m.chatModel.SetRoom("general", "General Chat", "", "")
		}
		wsURL := strings.Replace(m.client.GetBaseURL(), "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		return m, tea.Batch(
			m.chatModel.Init(),
			m.wsClient.Connect(wsURL, m.client.GetToken(), m.chatModel.RoomID()),
		)
	case "refresh":
		// Refresh current view
		switch m.currentView {
		case ViewDashboard:
			return m, m.dashboardModel.Init()
		case ViewLibrary:
			return m, m.libraryModel.Init()
		}
	case "quit":
		return m, tea.Quit
	case "back":
		if m.currentView != ViewDashboard {
			m.currentView = m.previousView
			if m.currentView == m.previousView {
				m.currentView = ViewDashboard
			}
		}
	}
	return m, nil
}

// View renders the UI
func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Build main layout
	content := m.renderCurrentView()
	footer := m.renderFooter()

	// Calculate available height for content
	footerHeight := lipgloss.Height(footer)
	contentHeight := m.height - footerHeight - 2 // padding

	// Render main container
	mainContent := lipgloss.NewStyle().
		Width(m.width).
		Height(contentHeight).
		Render(content)

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		mainContent,
		footer,
	)

	// Overlay rating modal if visible
	if m.showRating {
		ratingOverlay := m.ratingModal.View()
		if ratingOverlay != "" {
			return lipgloss.Place(
				m.width,
				m.height,
				lipgloss.Center,
				lipgloss.Center,
				ratingOverlay,
				lipgloss.WithWhitespaceChars(" "),
				lipgloss.WithWhitespaceForeground(lipgloss.Color("#222222")),
			)
		}
	}

	// Overlay comments view if visible
	if m.showComments {
		return m.commentsView.View()
	}

	// Overlay command palette if visible
	if m.paletteModel.IsVisible() {
		// Dim the background
		dimmed := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#777777")).
			Render(base)

		// Render palette on top
		overlay := m.paletteModel.View()

		// Stack them
		return dimmed + "\n" + overlay
	}

	return base
}

// =====================================
// VIEW RENDERING
// =====================================

// renderCurrentView renders the active view
func (m Model) renderCurrentView() string {
	header := m.renderHeader()

	var content string
	switch m.currentView {
	case ViewDashboard:
		content = m.dashboardModel.View()
	case ViewSearch:
		content = m.searchModel.View()
	case ViewLibrary:
		content = m.libraryModel.View()
	case ViewDetail:
		content = m.detailModel.View()
	case ViewBrowse:
		content = m.browseModel.View()
	case ViewActivity:
		content = m.activityModel.View()
	case ViewAuth:
		content = m.authModel.View()
	case ViewHelp:
		content = m.helpModel.View()
	case ViewChat:
		content = m.chatModel.View()
	default:
		content = "View not implemented"
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	title := m.theme.HeaderTitle.Render("📚 MangaHub Terminal")

	// User status
	var userStatus string
	if m.authenticated && m.user != nil {
		userStatus = m.theme.StatusOnline.Render("⚡ " + m.user.Username)
	} else {
		userStatus = m.theme.DimText.Render("○ Guest")
	}

	// Build header with title left, status right
	headerWidth := m.width - 4
	titleWidth := lipgloss.Width(title)
	statusWidth := lipgloss.Width(userStatus)
	padding := headerWidth - titleWidth - statusWidth
	if padding < 0 {
		padding = 0
	}

	header := title + lipgloss.NewStyle().Width(padding).Render("") + userStatus

	return m.theme.Header.Width(m.width).Render(header)
}

// renderFooter renders the bottom footer with keybindings
func (m Model) renderFooter() string {
	// Simplified hints - encourage using command palette
	hints := []string{
		styles.RenderKeyHint("Ctrl+P", "commands"),
		styles.RenderKeyHint("?", "help"),
	}

	// Chat indicator with unread count
	if m.unreadChatCount > 0 {
		chatHint := fmt.Sprintf("💬 Chat (%d)", m.unreadChatCount)
		hints = append(hints, styles.RenderKeyHint("c", chatHint))
	} else {
		hints = append(hints, styles.RenderKeyHint("c", "chat"))
	}

	// Add context-specific hints
	if m.inputMode {
		hints = append(hints, styles.RenderKeyHint("Esc", "cancel"))
	} else {
		hints = append(hints, styles.RenderKeyHint("Esc", "back"))
	}

	hints = append(hints, styles.RenderKeyHint("q", "quit"))

	// Join hints with separator
	hintsStr := ""
	for i, hint := range hints {
		if i > 0 {
			hintsStr += m.theme.DimText.Render("  │  ")
		}
		hintsStr += hint
	}

	// Toast notification overlay
	var toastLine string
	if m.toast != nil && m.toast.Visible {
		toastLine = m.toast.View() + "\n"
	}

	// Error display
	var errorLine string
	if m.lastError != nil {
		errorLine = m.theme.ErrorText.Render("⚠ " + m.lastError.Error())
	}

	footer := m.theme.Footer.Width(m.width).Render(hintsStr)
	if toastLine != "" {
		footer = toastLine + footer
	}
	if errorLine != "" {
		footer = errorLine + "\n" + footer
	}

	return footer
}

// isInputFocused reports whether the active view has a focused text input/textarea.
func (m Model) isInputFocused() bool {
	switch m.currentView {
	case ViewSearch:
		return m.searchModel.IsInputFocused()
	case ViewAuth:
		return m.authModel.IsInputFocused()
	case ViewChat:
		return m.chatModel.IsInputFocused()
	default:
		return false
	}
}

// =====================================
// TOAST NOTIFICATION MODEL
// =====================================

// ToastModel represents a temporary notification popup
type ToastModel struct {
	Content  string
	Visible  bool
	Duration time.Duration
	timer    *time.Timer
}

// ToastTickMsg signals toast timer tick
type ToastTickMsg struct{}

// ToastShowMsg shows a toast notification
type ToastShowMsg struct {
	Content  string
	Duration time.Duration
}

// ToastHideMsg hides the toast
type ToastHideMsg struct{}

// NewToast creates a new toast model
func NewToast() *ToastModel {
	return &ToastModel{
		Duration: 3 * time.Second,
	}
}

// Show displays the toast with content
func (t *ToastModel) Show(content string, duration time.Duration) tea.Cmd {
	t.Content = content
	t.Visible = true
	if duration > 0 {
		t.Duration = duration
	} else {
		t.Duration = 3 * time.Second
	}

	return tea.Tick(t.Duration, func(time.Time) tea.Msg {
		return ToastHideMsg{}
	})
}

// Hide hides the toast
func (t *ToastModel) Hide() {
	t.Visible = false
	t.Content = ""
}

// Update handles toast messages
func (t *ToastModel) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case ToastHideMsg:
		t.Hide()
	}
	return nil
}

// View renders the toast notification
func (t *ToastModel) View() string {
	if !t.Visible {
		return ""
	}

	toastStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#FF8800")).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Padding(0, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF8800"))

	return toastStyle.Render("🔔 " + t.Content)
}

// =====================================
// ERROR TYPES
// =====================================

type authRequiredError struct{}

func (e *authRequiredError) Error() string {
	return "Authentication required. Please login first."
}
