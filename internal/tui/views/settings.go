// Package views - Settings View
// User preferences page with theme, shortcuts, and data export
// Layout:
//
//	┌── ⚙️ Settings ──────────────────────────────────────────┐
//	│ ┌─ Appearance ────────────────────────────────────────┐ │
//	│ │ Theme:       [■ Dracula] [Light] [Dark] [Nord]      │ │
//	│ │ Language:    [■ English] [Vietnamese] [Japanese]    │ │
//	│ ├─ Reading ───────────────────────────────────────────┤ │
//	│ │ Chapters/Page:  [20 ▼]                              │ │
//	│ │ Reading Dir:    [→ LTR] [← RTL]                     │ │
//	│ ├─ Privacy ───────────────────────────────────────────┤ │
//	│ │ [✓] Public Activity    [✓] Public Library          │ │
//	│ ├─ Data ──────────────────────────────────────────────┤ │
//	│ │ [Export Data]  [Reset to Defaults]                  │ │
//	│ └─────────────────────────────────────────────────────┘ │
//	└──────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// =====================================
// SETTINGS MODEL
// =====================================

// SettingsModel holds the settings view state
type SettingsModel struct {
	width  int
	height int
	theme  *styles.Theme

	// Data
	preferences *models.UserPreferences

	// Loading states
	loading   bool
	saving    bool
	lastError error
	message   string // Success message

	// Selection
	selectedSection int // 0=appearance, 1=reading, 2=privacy, 3=data
	selectedItem    int // Item within section

	// Components
	spinner spinner.Model

	// API client
	client *api.Client
}

// Setting sections
const (
	sectionAppearance = iota
	sectionReading
	sectionPrivacy
	sectionData
	sectionKeybindings
)

// NewSettingsModel creates a new settings model
func NewSettingsModel(client *api.Client) *SettingsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	return &SettingsModel{
		theme:   styles.DefaultTheme,
		spinner: s,
		client:  client,
		loading: true,
	}
}

// NewSettings creates a new settings model with default client
func NewSettings() SettingsModel {
	m := NewSettingsModel(api.GetClient())
	return *m
}

// =====================================
// MESSAGES
// =====================================

type SettingsLoadedMsg struct {
	Preferences *models.UserPreferences
}

type SettingsSavedMsg struct {
	Preferences *models.UserPreferences
}

type SettingsErrorMsg struct {
	Error error
}

type SettingsResetMsg struct{}

type ExportCompleteMsg struct {
	Filename string
}

// =====================================
// COMMANDS
// =====================================

func (m SettingsModel) loadSettings() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		prefs, err := m.client.GetPreferences(ctx)
		if err != nil {
			return SettingsErrorMsg{Error: err}
		}
		return SettingsLoadedMsg{Preferences: prefs}
	}
}

func (m SettingsModel) saveSettings() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		req := &models.UpdatePreferencesRequest{
			Theme:                &m.preferences.Theme,
			Language:             &m.preferences.Language,
			ChaptersPerPage:      &m.preferences.ChaptersPerPage,
			ReadingDirection:     &m.preferences.ReadingDirection,
			DefaultStatus:        &m.preferences.DefaultStatus,
			ShowSpoilers:         &m.preferences.ShowSpoilers,
			AutoSync:             &m.preferences.AutoSync,
			NotificationsEnabled: &m.preferences.NotificationsEnabled,
			ActivityPublic:       &m.preferences.ActivityPublic,
			LibraryPublic:        &m.preferences.LibraryPublic,
			Keybindings:          &m.preferences.Keybindings,
		}

		prefs, err := m.client.UpdatePreferences(ctx, req)
		if err != nil {
			return SettingsErrorMsg{Error: err}
		}
		return SettingsSavedMsg{Preferences: prefs}
	}
}

func (m SettingsModel) resetSettings() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := m.client.ResetPreferences(ctx)
		if err != nil {
			return SettingsErrorMsg{Error: err}
		}
		return SettingsResetMsg{}
	}
}

func (m SettingsModel) exportData() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		export, err := m.client.ExportUserData(ctx)
		if err != nil {
			return SettingsErrorMsg{Error: err}
		}
		return ExportCompleteMsg{Filename: export.Filename}
	}
}

// =====================================
// MODEL METHODS
// =====================================

func (m SettingsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadSettings(),
	)
}

func (m SettingsModel) Update(msg tea.Msg) (SettingsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		// Clear message on any keypress
		m.message = ""

		switch msg.String() {
		case "r":
			m.loading = true
			return m, m.loadSettings()
		case "j", "down":
			m.moveDown()
		case "k", "up":
			m.moveUp()
		case "h", "left":
			m.adjustValue(-1)
			return m, m.saveSettings()
		case "l", "right":
			m.adjustValue(1)
			return m, m.saveSettings()
		case "enter", " ":
			return m, m.handleAction()
		case "tab":
			m.selectedSection = (m.selectedSection + 1) % 5
			m.selectedItem = 0
		case "shift+tab":
			m.selectedSection = (m.selectedSection + 4) % 5
			m.selectedItem = 0
		}

	case SettingsLoadedMsg:
		m.loading = false
		m.preferences = msg.Preferences
		m.lastError = nil

	case SettingsSavedMsg:
		m.saving = false
		m.preferences = msg.Preferences
		m.message = "Settings saved!"

	case SettingsResetMsg:
		m.message = "Settings reset to defaults!"
		return m, m.loadSettings()

	case ExportCompleteMsg:
		m.message = fmt.Sprintf("Data exported to %s", msg.Filename)

	case SettingsErrorMsg:
		m.loading = false
		m.saving = false
		m.lastError = msg.Error

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *SettingsModel) moveDown() {
	maxItems := m.getMaxItems()
	if m.selectedItem < maxItems-1 {
		m.selectedItem++
	} else {
		// Move to next section
		if m.selectedSection < 4 {
			m.selectedSection++
			m.selectedItem = 0
		}
	}
}

func (m *SettingsModel) moveUp() {
	if m.selectedItem > 0 {
		m.selectedItem--
	} else if m.selectedSection > 0 {
		m.selectedSection--
		m.selectedItem = m.getMaxItems() - 1
	}
}

func (m SettingsModel) getMaxItems() int {
	switch m.selectedSection {
	case sectionAppearance:
		return 2 // theme, language
	case sectionReading:
		return 3 // chapters/page, direction, default status
	case sectionPrivacy:
		return 4 // activity, library, notifications, spoilers
	case sectionData:
		return 2 // export, reset
	case sectionKeybindings:
		return 8 // various keybindings
	default:
		return 1
	}
}

func (m *SettingsModel) adjustValue(delta int) {
	if m.preferences == nil {
		return
	}

	switch m.selectedSection {
	case sectionAppearance:
		switch m.selectedItem {
		case 0: // Theme
			themes := []string{"dracula", "dark", "light", "nord"}
			idx := m.findIndex(themes, m.preferences.Theme)
			idx = (idx + delta + len(themes)) % len(themes)
			m.preferences.Theme = themes[idx]
		case 1: // Language
			langs := []string{"en", "vi", "jp"}
			idx := m.findIndex(langs, m.preferences.Language)
			idx = (idx + delta + len(langs)) % len(langs)
			m.preferences.Language = langs[idx]
		}
	case sectionReading:
		switch m.selectedItem {
		case 0: // Chapters per page
			m.preferences.ChaptersPerPage += delta * 5
			if m.preferences.ChaptersPerPage < 5 {
				m.preferences.ChaptersPerPage = 5
			}
			if m.preferences.ChaptersPerPage > 100 {
				m.preferences.ChaptersPerPage = 100
			}
		case 1: // Reading direction
			if m.preferences.ReadingDirection == "ltr" {
				m.preferences.ReadingDirection = "rtl"
			} else {
				m.preferences.ReadingDirection = "ltr"
			}
		case 2: // Default status
			statuses := []string{"reading", "plan_to_read"}
			idx := m.findIndex(statuses, m.preferences.DefaultStatus)
			idx = (idx + delta + len(statuses)) % len(statuses)
			m.preferences.DefaultStatus = statuses[idx]
		}
	case sectionPrivacy:
		switch m.selectedItem {
		case 0:
			m.preferences.ActivityPublic = !m.preferences.ActivityPublic
		case 1:
			m.preferences.LibraryPublic = !m.preferences.LibraryPublic
		case 2:
			m.preferences.NotificationsEnabled = !m.preferences.NotificationsEnabled
		case 3:
			m.preferences.ShowSpoilers = !m.preferences.ShowSpoilers
		}
	}
}

func (m SettingsModel) findIndex(arr []string, val string) int {
	for i, v := range arr {
		if v == val {
			return i
		}
	}
	return 0
}

func (m SettingsModel) handleAction() tea.Cmd {
	switch m.selectedSection {
	case sectionData:
		switch m.selectedItem {
		case 0:
			return m.exportData()
		case 1:
			return m.resetSettings()
		}
	case sectionPrivacy:
		m.adjustValue(1)
		return m.saveSettings()
	}
	return nil
}

// SetWidth sets the view width
func (m *SettingsModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *SettingsModel) SetHeight(h int) {
	m.height = h
}

func (m SettingsModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.loading {
		return m.renderLoading()
	}

	if m.lastError != nil {
		return m.renderError()
	}

	sections := []string{
		m.renderHeader(),
		m.renderAppearanceSection(),
		m.renderReadingSection(),
		m.renderPrivacySection(),
		m.renderDataSection(),
		m.renderKeybindingsSection(),
		m.renderFooter(),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return m.theme.Container.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(content)
}

// =====================================
// RENDER HELPERS
// =====================================

func (m SettingsModel) renderLoading() string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		m.spinner.View(),
		m.theme.DimText.Render("Loading settings..."),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m SettingsModel) renderError() string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		m.theme.ErrorText.Render("⚠ Failed to load settings"),
		m.theme.DimText.Render(m.lastError.Error()),
		"",
		m.theme.DimText.Render("[r] Retry"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m SettingsModel) renderHeader() string {
	title := m.theme.Title.Render("⚙️ Settings")

	statusLine := ""
	if m.message != "" {
		statusLine = m.theme.Success.Render("✓ " + m.message)
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, statusLine, "")
}

func (m SettingsModel) renderAppearanceSection() string {
	isActive := m.selectedSection == sectionAppearance

	header := m.renderSectionHeader("🎨 Appearance", isActive)

	// Theme option
	themeLabel := "Theme:"
	themeOptions := m.renderOptions(
		[]string{"Dracula", "Dark", "Light", "Nord"},
		[]string{"dracula", "dark", "light", "nord"},
		m.preferences.Theme,
		isActive && m.selectedItem == 0,
	)
	themeLine := fmt.Sprintf("  %-15s %s", themeLabel, themeOptions)

	// Language option
	langLabel := "Language:"
	langOptions := m.renderOptions(
		[]string{"English", "Tiếng Việt", "日本語"},
		[]string{"en", "vi", "jp"},
		m.preferences.Language,
		isActive && m.selectedItem == 1,
	)
	langLine := fmt.Sprintf("  %-15s %s", langLabel, langOptions)

	return lipgloss.JoinVertical(lipgloss.Left, header, themeLine, langLine, "")
}

func (m SettingsModel) renderReadingSection() string {
	isActive := m.selectedSection == sectionReading

	header := m.renderSectionHeader("📖 Reading", isActive)

	// Chapters per page
	chapterLabel := "Chapters/Page:"
	chapterValue := fmt.Sprintf("<%d>", m.preferences.ChaptersPerPage)
	if isActive && m.selectedItem == 0 {
		chapterValue = m.theme.Primary.Render(chapterValue)
	}
	chapterLine := fmt.Sprintf("  %-15s %s", chapterLabel, chapterValue)

	// Reading direction
	dirLabel := "Direction:"
	dirOptions := m.renderOptions(
		[]string{"→ LTR", "← RTL"},
		[]string{"ltr", "rtl"},
		m.preferences.ReadingDirection,
		isActive && m.selectedItem == 1,
	)
	dirLine := fmt.Sprintf("  %-15s %s", dirLabel, dirOptions)

	// Default status
	statusLabel := "Default Status:"
	statusOptions := m.renderOptions(
		[]string{"Reading", "Plan to Read"},
		[]string{"reading", "plan_to_read"},
		m.preferences.DefaultStatus,
		isActive && m.selectedItem == 2,
	)
	statusLine := fmt.Sprintf("  %-15s %s", statusLabel, statusOptions)

	return lipgloss.JoinVertical(lipgloss.Left, header, chapterLine, dirLine, statusLine, "")
}

func (m SettingsModel) renderPrivacySection() string {
	isActive := m.selectedSection == sectionPrivacy

	header := m.renderSectionHeader("🔒 Privacy & Notifications", isActive)

	activityLine := m.renderToggle("Public Activity", m.preferences.ActivityPublic, isActive && m.selectedItem == 0)
	libraryLine := m.renderToggle("Public Library", m.preferences.LibraryPublic, isActive && m.selectedItem == 1)
	notifLine := m.renderToggle("Notifications", m.preferences.NotificationsEnabled, isActive && m.selectedItem == 2)
	spoilerLine := m.renderToggle("Show Spoilers", m.preferences.ShowSpoilers, isActive && m.selectedItem == 3)

	return lipgloss.JoinVertical(lipgloss.Left, header, activityLine, libraryLine, notifLine, spoilerLine, "")
}

func (m SettingsModel) renderDataSection() string {
	isActive := m.selectedSection == sectionData

	header := m.renderSectionHeader("💾 Data", isActive)

	exportBtn := m.renderButton("📤 Export Data", isActive && m.selectedItem == 0)
	resetBtn := m.renderButton("🔄 Reset to Defaults", isActive && m.selectedItem == 1)

	return lipgloss.JoinVertical(lipgloss.Left, header, "  "+exportBtn+"  "+resetBtn, "")
}

func (m SettingsModel) renderKeybindingsSection() string {
	isActive := m.selectedSection == sectionKeybindings

	header := m.renderSectionHeader("⌨️ Keybindings", isActive)

	kb := m.preferences.Keybindings
	bindings := []string{
		fmt.Sprintf("  Next Chapter:     [%s]", kb.NextChapter),
		fmt.Sprintf("  Prev Chapter:     [%s]", kb.PrevChapter),
		fmt.Sprintf("  Toggle Favorite:  [%s]", kb.ToggleFavorite),
		fmt.Sprintf("  Open Search:      [%s]", kb.OpenSearch),
	}

	note := m.theme.DimText.Render("  (Keybindings are currently read-only)")

	return lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, append(bindings, note)...)...)
}

func (m SettingsModel) renderFooter() string {
	help := m.theme.DimText.Render("[↑↓] Navigate  [←→/Enter] Change  [Tab] Section  [r] Reload")
	return "\n" + help
}

// Helper methods

func (m SettingsModel) renderSectionHeader(title string, isActive bool) string {
	if isActive {
		return m.theme.Subtitle.Render("▶ " + title)
	}
	return m.theme.DimText.Render("  " + title)
}

func (m SettingsModel) renderOptions(labels, values []string, selected string, isActive bool) string {
	var opts []string
	for i, label := range labels {
		isSelected := values[i] == selected
		if isSelected {
			if isActive {
				opts = append(opts, m.theme.Primary.Render("[■ "+label+"]"))
			} else {
				opts = append(opts, "[■ "+label+"]")
			}
		} else {
			opts = append(opts, m.theme.DimText.Render("["+label+"]"))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, opts...)
}

func (m SettingsModel) renderToggle(label string, enabled bool, isActive bool) string {
	checkbox := "[ ]"
	if enabled {
		checkbox = "[✓]"
	}

	line := fmt.Sprintf("  %s %s", checkbox, label)
	if isActive {
		return m.theme.Primary.Render("▶" + line)
	}
	return " " + line
}

func (m SettingsModel) renderButton(label string, isActive bool) string {
	if isActive {
		return m.theme.ButtonActive.Render("[" + label + "]")
	}
	return m.theme.Button.Render("[" + label + "]")
}
