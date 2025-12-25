// Package views - Progress Update View
// Chapter progress tracking interface
// Layout:
//
//	┌───────────────────────────────────────────────────────┐
//	│  📖 UPDATE PROGRESS                                   │
//	│                                                       │
//	│  ONE PIECE                                            │
//	│  ────────────────────────────────────────────────     │
//	│                                                       │
//	│  CURRENT PROGRESS                                     │
//	│  Chapter: [  1093  ] / 1120                           │
//	│  [████████████████░░░░] 97.6%                         │
//	│                                                       │
//	│  READING STATUS                                       │
//	│  ┌──────────┐ ┌──────────┐ ┌──────────┐               │
//	│  │ READING  │ │ PLANNING │ │COMPLETED │               │
//	│  └──────────┘ └──────────┘ └──────────┘               │
//	│                                                       │
//	│  [Enter] Save   [Esc] Cancel   [+/-] Adjust           │
//	└───────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
)

// =====================================
// STATUS DEFINITIONS
// =====================================

// ReadingStatus options
var ReadingStatuses = []string{
	"reading",
	"planning",
	"completed",
	"on-hold",
	"dropped",
}

// StatusIcons maps status to display icons
var StatusIcons = map[string]string{
	"reading":   "📖",
	"planning":  "📋",
	"completed": "✅",
	"on-hold":   "⏸️",
	"dropped":   "❌",
}

// StatusLabels maps status to display labels
var StatusLabels = map[string]string{
	"reading":   "Reading",
	"planning":  "Plan to Read",
	"completed": "Completed",
	"on-hold":   "On Hold",
	"dropped":   "Dropped",
}

// =====================================
// PROGRESS MODEL
// =====================================

// ProgressModel holds the progress update view state
type ProgressModel struct {
	// Window dimensions
	width  int
	height int

	// Theme
	theme *styles.Theme

	// Manga info
	mangaID       string
	mangaTitle    string
	totalChapters int

	// Current values
	currentChapter int
	currentStatus  int // index into ReadingStatuses

	// Input
	chapterInput textinput.Model

	// UI state
	focused  int // 0 = chapter input, 1 = status selection
	saving   bool
	saved    bool
	errorMsg string

	// Components
	spinner spinner.Model

	// API client
	client *api.Client
}

// =====================================
// MESSAGES
// =====================================

// ProgressSavedMsg signals progress was saved
type ProgressSavedMsg struct {
	Chapter int
	Status  string
}

// ProgressErrorMsg signals an error
type ProgressErrorMsg struct {
	Error error
}

// =====================================
// CONSTRUCTOR
// =====================================

// NewProgress creates a new progress update model
func NewProgress(mangaID, mangaTitle string, currentChapter, totalChapters int, currentStatus string) ProgressModel {
	// Create chapter input
	ti := textinput.New()
	ti.Placeholder = "Chapter number"
	ti.Focus()
	ti.CharLimit = 10
	ti.Width = 10
	ti.SetValue(strconv.Itoa(currentChapter))
	ti.PromptStyle = styles.DefaultTheme.Primary
	ti.TextStyle = styles.DefaultTheme.Description

	// Create spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	// Find current status index
	statusIdx := 0
	for i, status := range ReadingStatuses {
		if status == currentStatus {
			statusIdx = i
			break
		}
	}

	return ProgressModel{
		theme:          styles.DefaultTheme,
		mangaID:        mangaID,
		mangaTitle:     mangaTitle,
		currentChapter: currentChapter,
		totalChapters:  totalChapters,
		currentStatus:  statusIdx,
		chapterInput:   ti,
		spinner:        s,
		client:         api.GetClient(),
		focused:        0,
	}
}

// =====================================
// BUBBLE TEA INTERFACE
// =====================================

// Init initializes the progress view
func (m ProgressModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages
func (m ProgressModel) Update(msg tea.Msg) (ProgressModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if m.saving {
			return m, nil
		}

		switch msg.String() {
		case "tab":
			// Toggle focus
			m.focused = (m.focused + 1) % 2
			if m.focused == 0 {
				m.chapterInput.Focus()
			} else {
				m.chapterInput.Blur()
			}

		case "left", "h":
			if m.focused == 1 {
				m.currentStatus--
				if m.currentStatus < 0 {
					m.currentStatus = len(ReadingStatuses) - 1
				}
			}

		case "right", "l":
			if m.focused == 1 {
				m.currentStatus = (m.currentStatus + 1) % len(ReadingStatuses)
			}

		case "+", "=":
			if m.focused == 0 {
				current, _ := strconv.Atoi(m.chapterInput.Value())
				current++
				if m.totalChapters > 0 && current > m.totalChapters {
					current = m.totalChapters
				}
				m.chapterInput.SetValue(strconv.Itoa(current))
			}

		case "-", "_":
			if m.focused == 0 {
				current, _ := strconv.Atoi(m.chapterInput.Value())
				current--
				if current < 0 {
					current = 0
				}
				m.chapterInput.SetValue(strconv.Itoa(current))
			}

		case "enter":
			return m, m.saveProgress()

		default:
			if m.focused == 0 {
				// Update text input
				var cmd tea.Cmd
				m.chapterInput, cmd = m.chapterInput.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case ProgressSavedMsg:
		m.saving = false
		m.saved = true
		m.currentChapter = msg.Chapter

	case ProgressErrorMsg:
		m.saving = false
		m.errorMsg = msg.Error.Error()

	case spinner.TickMsg:
		if m.saving {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// saveProgress saves the progress to the API
func (m ProgressModel) saveProgress() tea.Cmd {
	m.saving = true
	return func() tea.Msg {
		ctx := context.Background()

		chapter, err := strconv.Atoi(m.chapterInput.Value())
		if err != nil {
			chapter = m.currentChapter
		}

		status := ReadingStatuses[m.currentStatus]

		// Update progress with chapter, status, and favorite flag
		err = m.client.UpdateProgress(ctx, m.mangaID, chapter, status, false)
		if err != nil {
			return ProgressErrorMsg{Error: err}
		}

		return ProgressSavedMsg{
			Chapter: chapter,
			Status:  status,
		}
	}
}

// View renders the progress view
func (m ProgressModel) View() string {
	var sections []string

	// ===== HEADER =====
	header := m.theme.PanelHeader.Render("📖 UPDATE PROGRESS")
	sections = append(sections, header+"\n")

	// ===== MANGA TITLE =====
	title := m.theme.Title.Render(m.mangaTitle)
	separator := m.theme.DimText.Render(strings.Repeat("─", min(lipgloss.Width(title), m.width-10)))
	sections = append(sections, title+"\n"+separator+"\n")

	// ===== CHAPTER INPUT =====
	chapterSection := m.renderChapterSection()
	sections = append(sections, chapterSection+"\n")

	// ===== STATUS SELECTION =====
	statusSection := m.renderStatusSection()
	sections = append(sections, statusSection+"\n")

	// ===== FEEDBACK =====
	if m.saving {
		feedback := m.spinner.View() + " Saving..."
		sections = append(sections, m.theme.DimText.Render(feedback)+"\n")
	} else if m.saved {
		feedback := m.theme.Success.Render("✓ Progress saved!")
		sections = append(sections, feedback+"\n")
	} else if m.errorMsg != "" {
		feedback := m.theme.ErrorText.Render("✗ " + m.errorMsg)
		sections = append(sections, feedback+"\n")
	}

	// ===== HELP =====
	help := m.renderHelp()
	sections = append(sections, help)

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.theme.CardFocused.Width(m.width - 4).Render(content)
}

// =====================================
// RENDERERS
// =====================================

func (m ProgressModel) renderChapterSection() string {
	header := m.theme.PanelHeader.Render("CURRENT PROGRESS")

	// Chapter input row
	var inputStyle lipgloss.Style
	if m.focused == 0 {
		inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorPrimary).
			Padding(0, 1)
	} else {
		inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorDim).
			Padding(0, 1)
	}

	inputBox := inputStyle.Render(m.chapterInput.View())

	var totalText string
	if m.totalChapters > 0 {
		totalText = m.theme.DimText.Render(fmt.Sprintf(" / %d", m.totalChapters))
	}

	chapterRow := "Chapter: " + inputBox + totalText

	// Progress bar
	chapter, _ := strconv.Atoi(m.chapterInput.Value())
	var progressPct float64
	if m.totalChapters > 0 {
		progressPct = float64(chapter) / float64(m.totalChapters)
		if progressPct > 1.0 {
			progressPct = 1.0
		}
	}

	progressBar := styles.RenderProgressBar(progressPct, 30)
	progressPctText := m.theme.Primary.Render(fmt.Sprintf(" %.1f%%", progressPct*100))

	return header + "\n" + chapterRow + "\n" + progressBar + progressPctText
}

func (m ProgressModel) renderStatusSection() string {
	header := m.theme.PanelHeader.Render("READING STATUS")

	var buttons []string
	for i, status := range ReadingStatuses {
		label := StatusLabels[status]
		icon := StatusIcons[status]

		var style lipgloss.Style
		if i == m.currentStatus {
			if m.focused == 1 {
				style = m.theme.ButtonActive
			} else {
				style = lipgloss.NewStyle().
					Background(styles.ColorPrimary).
					Foreground(styles.ColorBackground).
					Padding(0, 1).
					Bold(true)
			}
		} else {
			style = m.theme.ButtonInactive
		}

		btn := style.Render(fmt.Sprintf("%s %s", icon, label))
		buttons = append(buttons, btn)
	}

	buttonRow := lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
	return header + "\n" + buttonRow
}

func (m ProgressModel) renderHelp() string {
	helpItems := []string{
		m.theme.Key.Render("[Enter]") + " " + m.theme.DimText.Render("Save"),
		m.theme.Key.Render("[Esc]") + " " + m.theme.DimText.Render("Cancel"),
		m.theme.Key.Render("[Tab]") + " " + m.theme.DimText.Render("Switch"),
		m.theme.Key.Render("[+/-]") + " " + m.theme.DimText.Render("Adjust"),
		m.theme.Key.Render("[←→]") + " " + m.theme.DimText.Render("Status"),
	}
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Center, helpItems...)
}

// =====================================
// PUBLIC METHODS
// =====================================

// SetManga sets the manga to update progress for
func (m *ProgressModel) SetManga(mangaID, mangaTitle string, currentChapter, totalChapters int, currentStatus string) {
	m.mangaID = mangaID
	m.mangaTitle = mangaTitle
	m.currentChapter = currentChapter
	m.totalChapters = totalChapters
	m.chapterInput.SetValue(strconv.Itoa(currentChapter))
	m.saved = false
	m.errorMsg = ""

	// Find status index
	for i, status := range ReadingStatuses {
		if status == currentStatus {
			m.currentStatus = i
			break
		}
	}
}

// IsSaved returns true if progress was saved
func (m ProgressModel) IsSaved() bool {
	return m.saved
}

// GetChapter returns the current chapter value
func (m ProgressModel) GetChapter() int {
	chapter, _ := strconv.Atoi(m.chapterInput.Value())
	return chapter
}

// GetStatus returns the current status value
func (m ProgressModel) GetStatus() string {
	return ReadingStatuses[m.currentStatus]
}

// SetWidth sets the view width
func (m *ProgressModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *ProgressModel) SetHeight(h int) {
	m.height = h
}
