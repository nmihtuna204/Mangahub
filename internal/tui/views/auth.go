// Package views - Authentication View
// Login and Signup forms for MangaHub TUI
// Layout:
//
//	┌── 🔐 MangaHub Login ───────────────────────────────────┐
//	│                                                        │
//	│              📚 Welcome to MangaHub                    │
//	│           Your Manga Reading Terminal                  │
//	│                                                        │
//	│  ┌─────────────────────────────────────────────────┐   │
//	│  │ Username: [________________]                    │   │
//	│  │ Password: [________________]                    │   │
//	│  │                                                 │   │
//	│  │        [ Login ]    [ Sign Up ]                 │   │
//	│  └─────────────────────────────────────────────────┘   │
//	│                                                        │
//	│  [Tab] Switch field  [Enter] Submit  [Esc] Guest mode  │
//	└────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// =====================================
// AUTH MODEL
// =====================================

// AuthMode represents login or signup mode
type AuthMode int

const (
	ModeLogin AuthMode = iota
	ModeSignup
)

// AuthModel holds the authentication view state
type AuthModel struct {
	width  int
	height int
	theme  *styles.Theme

	// Form state
	mode         AuthMode
	focusedField int // 0=username, 1=email (signup only), 2=password, 3=confirm (signup only)

	// Text inputs
	usernameInput textinput.Model
	emailInput    textinput.Model
	passwordInput textinput.Model
	confirmInput  textinput.Model

	// State
	loading   bool
	lastError string
	message   string
	loggedIn  bool
	user      *models.User

	// Components
	spinner spinner.Model

	// API client
	client *api.Client
}

// NewAuthModel creates a new auth model
func NewAuthModel(client *api.Client) *AuthModel {
	// Username input
	usernameInput := textinput.New()
	usernameInput.Placeholder = "Enter username"
	usernameInput.CharLimit = 32
	usernameInput.Width = 30
	usernameInput.Focus()

	// Email input (for signup)
	emailInput := textinput.New()
	emailInput.Placeholder = "Enter email"
	emailInput.CharLimit = 64
	emailInput.Width = 30

	// Password input
	passwordInput := textinput.New()
	passwordInput.Placeholder = "Enter password"
	passwordInput.CharLimit = 64
	passwordInput.Width = 30
	passwordInput.EchoMode = textinput.EchoPassword
	passwordInput.EchoCharacter = '•'

	// Confirm password input (for signup)
	confirmInput := textinput.New()
	confirmInput.Placeholder = "Confirm password"
	confirmInput.CharLimit = 64
	confirmInput.Width = 30
	confirmInput.EchoMode = textinput.EchoPassword
	confirmInput.EchoCharacter = '•'

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	return &AuthModel{
		theme:         styles.DefaultTheme,
		mode:          ModeLogin,
		usernameInput: usernameInput,
		emailInput:    emailInput,
		passwordInput: passwordInput,
		confirmInput:  confirmInput,
		spinner:       s,
		client:        client,
	}
}

// NewAuth creates a new auth model with default client
func NewAuth() AuthModel {
	m := NewAuthModel(api.GetClient())
	return *m
}

// =====================================
// MESSAGES
// =====================================

// AuthSuccessMsg signals successful authentication
type AuthSuccessMsg struct {
	User *models.User
}

// AuthErrorMsg signals authentication failure
type AuthErrorMsg struct {
	Error string
}

// =====================================
// COMMANDS
// =====================================

func (m AuthModel) doLogin() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		username := strings.TrimSpace(m.usernameInput.Value())
		password := m.passwordInput.Value()

		if username == "" || password == "" {
			return AuthErrorMsg{Error: "Username and password are required"}
		}

		user, err := m.client.Login(ctx, username, password)
		if err != nil {
			return AuthErrorMsg{Error: err.Error()}
		}

		return AuthSuccessMsg{User: user}
	}
}

func (m AuthModel) doRegister() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		username := strings.TrimSpace(m.usernameInput.Value())
		email := strings.TrimSpace(m.emailInput.Value())
		password := m.passwordInput.Value()
		confirm := m.confirmInput.Value()

		// Validation
		if username == "" {
			return AuthErrorMsg{Error: "Username is required"}
		}
		if len(username) < 3 {
			return AuthErrorMsg{Error: "Username must be at least 3 characters"}
		}
		if email == "" {
			return AuthErrorMsg{Error: "Email is required"}
		}
		if !strings.Contains(email, "@") {
			return AuthErrorMsg{Error: "Invalid email format"}
		}
		if password == "" {
			return AuthErrorMsg{Error: "Password is required"}
		}
		if len(password) < 6 {
			return AuthErrorMsg{Error: "Password must be at least 6 characters"}
		}
		if password != confirm {
			return AuthErrorMsg{Error: "Passwords do not match"}
		}

		user, err := m.client.Register(ctx, username, email, password)
		if err != nil {
			return AuthErrorMsg{Error: err.Error()}
		}

		return AuthSuccessMsg{User: user}
	}
}

// =====================================
// MODEL METHODS
// =====================================

func (m AuthModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m AuthModel) Update(msg tea.Msg) (AuthModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Clear error on keypress
		m.lastError = ""

		switch msg.String() {
		case "ctrl+c", "esc":
			// Allow guest mode - return nil to signal skip
			return m, func() tea.Msg {
				return AuthSkipMsg{}
			}

		case "tab", "shift+tab":
			// Cycle through fields
			if m.mode == ModeLogin {
				m.focusedField = (m.focusedField + 1) % 2
			} else {
				if msg.String() == "shift+tab" {
					m.focusedField = (m.focusedField + 3) % 4
				} else {
					m.focusedField = (m.focusedField + 1) % 4
				}
			}
			m.updateFocus()

		case "enter":
			if m.loading {
				return m, nil
			}
			m.loading = true
			if m.mode == ModeLogin {
				return m, tea.Batch(m.spinner.Tick, m.doLogin())
			}
			return m, tea.Batch(m.spinner.Tick, m.doRegister())

		case "ctrl+s":
			// Toggle between login and signup
			if m.mode == ModeLogin {
				m.mode = ModeSignup
				m.focusedField = 0
			} else {
				m.mode = ModeLogin
				m.focusedField = 0
			}
			m.updateFocus()
			return m, nil

		default:
			// Update focused input
			var cmd tea.Cmd
			switch m.focusedField {
			case 0:
				m.usernameInput, cmd = m.usernameInput.Update(msg)
			case 1:
				if m.mode == ModeSignup {
					m.emailInput, cmd = m.emailInput.Update(msg)
				} else {
					m.passwordInput, cmd = m.passwordInput.Update(msg)
				}
			case 2:
				if m.mode == ModeSignup {
					m.passwordInput, cmd = m.passwordInput.Update(msg)
				}
			case 3:
				if m.mode == ModeSignup {
					m.confirmInput, cmd = m.confirmInput.Update(msg)
				}
			}
			cmds = append(cmds, cmd)
		}

	case AuthSuccessMsg:
		m.loading = false
		m.loggedIn = true
		m.user = msg.User
		m.message = "Login successful! Welcome, " + msg.User.Username
		// This will be handled by app.go
		return m, func() tea.Msg { return msg }

	case AuthErrorMsg:
		m.loading = false
		m.lastError = msg.Error

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *AuthModel) updateFocus() {
	m.usernameInput.Blur()
	m.emailInput.Blur()
	m.passwordInput.Blur()
	m.confirmInput.Blur()

	switch m.focusedField {
	case 0:
		m.usernameInput.Focus()
	case 1:
		if m.mode == ModeSignup {
			m.emailInput.Focus()
		} else {
			m.passwordInput.Focus()
		}
	case 2:
		if m.mode == ModeSignup {
			m.passwordInput.Focus()
		}
	case 3:
		if m.mode == ModeSignup {
			m.confirmInput.Focus()
		}
	}
}

func (m AuthModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	// Build the form
	var content strings.Builder

	// Header
	title := m.theme.Title.Render("📚 Welcome to MangaHub")
	subtitle := m.theme.DimText.Render("Your Manga Reading Terminal")
	content.WriteString(lipgloss.JoinVertical(lipgloss.Center, title, subtitle))
	content.WriteString("\n\n")

	// Mode indicator
	var modeText string
	if m.mode == ModeLogin {
		modeText = m.theme.Primary.Render("🔐 Login")
	} else {
		modeText = m.theme.Primary.Render("📝 Sign Up")
	}
	content.WriteString(lipgloss.PlaceHorizontal(40, lipgloss.Center, modeText))
	content.WriteString("\n\n")

	// Form fields
	fieldStyle := lipgloss.NewStyle().Width(40)
	labelStyle := m.theme.Subtitle.Width(12)

	// Username
	usernameLabel := labelStyle.Render("Username:")
	content.WriteString(fieldStyle.Render(usernameLabel + " " + m.usernameInput.View()))
	content.WriteString("\n\n")

	// Email (signup only)
	if m.mode == ModeSignup {
		emailLabel := labelStyle.Render("Email:")
		content.WriteString(fieldStyle.Render(emailLabel + " " + m.emailInput.View()))
		content.WriteString("\n\n")
	}

	// Password
	passwordLabel := labelStyle.Render("Password:")
	content.WriteString(fieldStyle.Render(passwordLabel + " " + m.passwordInput.View()))
	content.WriteString("\n\n")

	// Confirm password (signup only)
	if m.mode == ModeSignup {
		confirmLabel := labelStyle.Render("Confirm:")
		content.WriteString(fieldStyle.Render(confirmLabel + " " + m.confirmInput.View()))
		content.WriteString("\n\n")
	}

	// Loading indicator
	if m.loading {
		content.WriteString(lipgloss.PlaceHorizontal(40, lipgloss.Center,
			m.spinner.View()+" Authenticating..."))
		content.WriteString("\n\n")
	}

	// Error message
	if m.lastError != "" {
		errorBox := m.theme.ErrorText.
			Padding(0, 1).
			Render("⚠ " + m.lastError)
		content.WriteString(lipgloss.PlaceHorizontal(40, lipgloss.Center, errorBox))
		content.WriteString("\n\n")
	}

	// Success message
	if m.message != "" {
		successBox := m.theme.SuccessText.
			Padding(0, 1).
			Render("✓ " + m.message)
		content.WriteString(lipgloss.PlaceHorizontal(40, lipgloss.Center, successBox))
		content.WriteString("\n\n")
	}

	// Help text
	var helpLines []string
	helpLines = append(helpLines, m.theme.DimText.Render("[Tab] Next field"))
	helpLines = append(helpLines, m.theme.DimText.Render("[Enter] Submit"))
	if m.mode == ModeLogin {
		helpLines = append(helpLines, m.theme.DimText.Render("[Ctrl+S] Switch to Sign Up"))
	} else {
		helpLines = append(helpLines, m.theme.DimText.Render("[Ctrl+S] Switch to Login"))
	}
	helpLines = append(helpLines, m.theme.DimText.Render("[Esc] Continue as Guest"))

	helpText := strings.Join(helpLines, "  │  ")
	content.WriteString(lipgloss.PlaceHorizontal(m.width-10, lipgloss.Center, helpText))

	// Center the form in the window
	formBox := m.theme.Card.
		Width(50).
		Padding(2, 4).
		Render(content.String())

	return lipgloss.Place(
		m.width,
		m.height,
		lipgloss.Center,
		lipgloss.Center,
		formBox,
	)
}

// SetWidth sets the view width
func (m *AuthModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *AuthModel) SetHeight(h int) {
	m.height = h
}

// IsAuthenticated returns whether user logged in
func (m AuthModel) IsAuthenticated() bool {
	return m.client.IsAuthenticated()
}

// IsLoggedIn returns whether user just logged in successfully
func (m AuthModel) IsLoggedIn() bool {
	return m.loggedIn
}

// GetUser returns the logged in user
func (m AuthModel) GetUser() *models.User {
	return m.user
}

// IsInputFocused reports whether any auth field is focused.
func (m AuthModel) IsInputFocused() bool {
	return m.usernameInput.Focused() || m.emailInput.Focused() || m.passwordInput.Focused() || m.confirmInput.Focused()
}

// =====================================
// SKIP MESSAGE
// =====================================

// AuthSkipMsg signals user wants to continue as guest
type AuthSkipMsg struct{}
