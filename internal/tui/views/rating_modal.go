// Package views - Rating Modal Component
// Modal dialog for submitting manga ratings
package views

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
)

// RatingModal holds the rating modal state
type RatingModal struct {
	mangaID     string
	mangaTitle  string
	rating      float64 // 0.0 to 10.0
	review      textarea.Model
	active      bool
	submitting  bool
	spinner     spinner.Model
	lastError   error
	client      *api.Client
	width       int
	height      int
	theme       *styles.Theme
	focusReview bool // false = rating, true = review
}

// RatingSubmittedMsg signals rating was submitted
type RatingSubmittedMsg struct {
	MangaID string
	Rating  float64
}

// RatingErrorMsg signals rating submission failed
type RatingErrorMsg struct {
	Error error
}

// NewRatingModal creates a new rating modal
func NewRatingModal(mangaID, mangaTitle string) RatingModal {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	ta := textarea.New()
	ta.Placeholder = "Write your review (optional)..."
	ta.CharLimit = 1000
	ta.SetWidth(60)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false

	return RatingModal{
		mangaID:    mangaID,
		mangaTitle: mangaTitle,
		rating:     7.0, // Default to 7.0
		review:     ta,
		spinner:    s,
		client:     api.GetClient(),
		theme:      styles.DefaultTheme,
		active:     true,
	}
}

// Init initializes the modal
func (m RatingModal) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages
func (m RatingModal) Update(msg tea.Msg) (RatingModal, tea.Cmd) {
	var cmds []tea.Cmd

	if m.submitting {
		switch msg := msg.(type) {
		case spinner.TickMsg:
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		case RatingSubmittedMsg:
			m.submitting = false
			m.active = false
			return m, nil
		case RatingErrorMsg:
			m.lastError = msg.Error
			m.submitting = false
			return m, nil
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.focusReview {
			// Review textarea is focused
			switch msg.String() {
			case "esc":
				m.focusReview = false
				return m, nil
			case "ctrl+s":
				// Submit rating
				m.submitting = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.submitRating(),
				)
			default:
				var cmd tea.Cmd
				m.review, cmd = m.review.Update(msg)
				cmds = append(cmds, cmd)
			}
		} else {
			// Rating selection is focused
			switch msg.String() {
			case "esc", "q":
				m.active = false
				return m, nil
			case "left", "h":
				m.rating = maxFloat(0.0, m.rating-0.5)
			case "right", "l":
				m.rating = minFloat(10.0, m.rating+0.5)
			case "down", "j":
				m.rating = maxFloat(0.0, m.rating-1.0)
			case "up", "k":
				m.rating = minFloat(10.0, m.rating+1.0)
			case "tab":
				m.focusReview = true
				m.review.Focus()
				return m, textarea.Blink
			case "enter", "ctrl+s":
				// Submit rating
				m.submitting = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.submitRating(),
				)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}

	return m, tea.Batch(cmds...)
}

// submitRating submits the rating to the API
func (m RatingModal) submitRating() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.client.SubmitRating(ctx, m.mangaID, int(m.rating), m.review.Value())
		if err != nil {
			return RatingErrorMsg{Error: err}
		}
		return RatingSubmittedMsg{
			MangaID: m.mangaID,
			Rating:  m.rating,
		}
	}
}

// View renders the modal
func (m RatingModal) View() string {
	if !m.active {
		return ""
	}

	modalWidth := 70
	if m.width > 0 && m.width < 80 {
		modalWidth = m.width - 10
	}

	// Title
	title := m.theme.Title.Render(fmt.Sprintf("Rate: %s", m.mangaTitle))

	// Submitting state
	if m.submitting {
		content := lipgloss.NewStyle().
			Width(modalWidth).
			Align(lipgloss.Center).
			Render(m.spinner.View() + " Submitting rating...")
		return m.renderModal(title + "\n\n" + content)
	}

	// Error display
	var errorMsg string
	if m.lastError != nil {
		errorMsg = m.theme.ErrorText.Render(fmt.Sprintf("Error: %v", m.lastError)) + "\n\n"
	}

	// Rating slider
	ratingLabel := "Your Rating:"
	if !m.focusReview {
		ratingLabel = m.theme.Primary.Bold(true).Render("▶ Your Rating:")
	}

	ratingBar := m.renderRatingBar()
	ratingText := m.theme.Title.Render(fmt.Sprintf("%.1f / 10.0", m.rating))

	ratingSection := lipgloss.JoinVertical(
		lipgloss.Left,
		ratingLabel,
		ratingBar,
		ratingText,
	)

	// Review textarea
	reviewLabel := "\nReview (optional):"
	if m.focusReview {
		reviewLabel = m.theme.Primary.Bold(true).Render("\n▶ Review (optional):")
	}

	reviewSection := reviewLabel + "\n" + m.review.View()

	// Help text
	helpStyle := m.theme.DimText
	var helpText string
	if m.focusReview {
		helpText = helpStyle.Render("ESC: back to rating | Ctrl+S: submit | Tab: switch focus")
	} else {
		helpText = helpStyle.Render("←/→: adjust by 0.5 | ↑/↓: adjust by 1.0 | Tab: review | Enter: submit | ESC: cancel")
	}

	// Combine sections
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"\n",
		errorMsg,
		ratingSection,
		reviewSection,
		"\n",
		helpText,
	)

	return m.renderModal(content)
}

// renderRatingBar renders the visual rating bar
func (m RatingModal) renderRatingBar() string {
	const barWidth = 50
	filled := int((m.rating / 10.0) * float64(barWidth))

	var bar string
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += m.theme.Primary.Render("█")
		} else {
			bar += m.theme.DimText.Render("░")
		}
	}

	return "[" + bar + "]"
}

// renderModal wraps content in modal styling
func (m RatingModal) renderModal(content string) string {
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(1, 2).
		Width(70).
		Background(styles.ColorBackground)

	// Center on screen
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			modalStyle.Render(content),
		)
	}

	return modalStyle.Render(content)
}

// IsActive returns whether the modal is active
func (m RatingModal) IsActive() bool {
	return m.active
}

// Close closes the modal
func (m *RatingModal) Close() {
	m.active = false
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
