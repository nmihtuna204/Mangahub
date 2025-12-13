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

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
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
	ViewLogin
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
			return ViewChangeMsg{View: ViewLogin}
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
		m.searchModel.SetWidth(msg.Width - 4)
		m.searchModel.SetHeight(msg.Height - 6)
		m.libraryModel.SetWidth(msg.Width - 4)
		m.libraryModel.SetHeight(msg.Height - 6)
		m.browseModel.SetWidth(msg.Width - 4)
		m.browseModel.SetHeight(msg.Height - 6)
		m.activityModel.SetWidth(msg.Width - 4)
		m.activityModel.SetHeight(msg.Height - 6)
		return m, nil

	case tea.KeyMsg:
		// Global key handling
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
				m.lastError = &authRequiredError{}
				return m, nil
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

		case key.Matches(msg, m.keys.Back):
			if m.currentView != ViewDashboard {
				m.currentView = m.previousView
				if m.currentView == m.previousView {
					m.currentView = ViewDashboard
				}
			}
			return m, nil

		default:
			// Pass to current view
			return m.updateCurrentView(msg)
		}

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
		return m, nil

	case ErrorMsg:
		m.lastError = msg.Error
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
	}

	return m, cmd
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

	return lipgloss.JoinVertical(
		lipgloss.Left,
		mainContent,
		footer,
	)
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
	case ViewLogin:
		content = m.renderLoginPlaceholder()
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
	// Build help hints
	hints := []string{
		styles.RenderKeyHint("h", "home"),
		styles.RenderKeyHint("s", "search"),
		styles.RenderKeyHint("l", "library"),
		styles.RenderKeyHint("b", "browse"),
		styles.RenderKeyHint("a", "activity"),
		styles.RenderKeyHint("?", "help"),
		styles.RenderKeyHint("q", "quit"),
	}

	// Join hints with separator
	hintsStr := ""
	for i, hint := range hints {
		if i > 0 {
			hintsStr += m.theme.DimText.Render("  │  ")
		}
		hintsStr += hint
	}

	// Error display
	var errorLine string
	if m.lastError != nil {
		errorLine = m.theme.ErrorText.Render("⚠ " + m.lastError.Error())
	}

	footer := m.theme.Footer.Width(m.width).Render(hintsStr)
	if errorLine != "" {
		footer = errorLine + "\n" + footer
	}

	return footer
}

// =====================================
// PLACEHOLDER VIEWS (to be implemented)
// =====================================

func (m Model) renderLoginPlaceholder() string {
	return m.theme.Container.Width(m.width - 4).Render(
		m.theme.Title.Render("🔐 Login") + "\n\n" +
			"Please login to continue.\n\n" +
			"[Implementation coming in next phase]")
}

// =====================================
// ERROR TYPES
// =====================================

type authRequiredError struct{}

func (e *authRequiredError) Error() string {
	return "Authentication required. Please login first."
}
