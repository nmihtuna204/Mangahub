// Package views - Comments View Component
// Display and post comments for manga
package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// CommentsView holds the comments view state
type CommentsView struct {
	mangaID       string
	mangaTitle    string
	comments      []models.CommentWithReplies
	viewport      viewport.Model
	textarea      textarea.Model
	active        bool
	loading       bool
	posting       bool
	spinner       spinner.Model
	selectedIndex int
	composing     bool // Whether user is composing a comment
	lastError     error
	client        *api.Client
	width         int
	height        int
	theme         *styles.Theme
}

// CommentsLoadedMsg signals comments were loaded
type CommentsLoadedMsg struct {
	Comments []models.CommentWithReplies
}

// CommentPostedMsg signals comment was posted
type CommentPostedMsg struct{}

// CommentsErrorMsg signals an error
type CommentsErrorMsg struct {
	Error error
}

// NewCommentsView creates a new comments view
func NewCommentsView(mangaID, mangaTitle string) CommentsView {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = styles.DefaultTheme.Spinner

	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ColorPrimary).
		Padding(0, 1)

	ta := textarea.New()
	ta.Placeholder = "Write your comment..."
	ta.CharLimit = 500
	ta.SetWidth(76)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	return CommentsView{
		mangaID:    mangaID,
		mangaTitle: mangaTitle,
		viewport:   vp,
		textarea:   ta,
		spinner:    s,
		client:     api.GetClient(),
		theme:      styles.DefaultTheme,
		active:     true,
		loading:    true,
	}
}

// Init initializes the view
func (m CommentsView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadComments(),
	)
}

// loadComments loads comments from API
func (m CommentsView) loadComments() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.client.GetComments(ctx, m.mangaID, 1, 50)
		if err != nil {
			return CommentsErrorMsg{Error: err}
		}
		return CommentsLoadedMsg{Comments: result.Comments}
	}
}

// postComment posts a new comment
func (m CommentsView) postComment() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		content := m.textarea.Value()
		if content == "" {
			return CommentsErrorMsg{Error: fmt.Errorf("comment cannot be empty")}
		}

		err := m.client.PostComment(ctx, m.mangaID, content, nil, nil)
		if err != nil {
			return CommentsErrorMsg{Error: err}
		}
		return CommentPostedMsg{}
	}
}

// Update handles messages
func (m CommentsView) Update(msg tea.Msg) (CommentsView, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.composing {
			// Composing mode - textarea is focused
			switch msg.String() {
			case "esc":
				m.composing = false
				m.textarea.Blur()
				m.textarea.Reset()
				return m, nil
			case "ctrl+s":
				// Submit comment
				m.posting = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.postComment(),
				)
			default:
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				cmds = append(cmds, cmd)
			}
		} else {
			// Navigation mode
			switch msg.String() {
			case "esc", "q":
				m.active = false
				return m, nil
			case "up", "k":
				m.selectedIndex--
				if m.selectedIndex < 0 {
					m.selectedIndex = 0
				}
			case "down", "j":
				m.selectedIndex++
				if m.selectedIndex >= len(m.comments) {
					m.selectedIndex = len(m.comments) - 1
				}
			case "c":
				// Start composing
				m.composing = true
				m.textarea.Focus()
				return m, textarea.Blink
			case "l":
				// Like selected comment
				if m.selectedIndex >= 0 && m.selectedIndex < len(m.comments) {
					commentID := m.comments[m.selectedIndex].ID
					return m, m.likeComment(commentID)
				}
			case "r":
				// Refresh comments
				m.loading = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.loadComments(),
				)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 8
		m.viewport.Height = msg.Height - 15
		m.textarea.SetWidth(msg.Width - 12)

	case CommentsLoadedMsg:
		m.comments = msg.Comments
		m.loading = false
		m.viewport.SetContent(m.renderCommentsList())

	case CommentPostedMsg:
		m.posting = false
		m.composing = false
		m.textarea.Reset()
		m.textarea.Blur()
		// Reload comments
		m.loading = true
		return m, tea.Batch(
			m.spinner.Tick,
			m.loadComments(),
		)

	case CommentsErrorMsg:
		m.lastError = msg.Error
		m.loading = false
		m.posting = false

	case spinner.TickMsg:
		if m.loading || m.posting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Update viewport
	if !m.composing {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// likeComment likes a comment
func (m CommentsView) likeComment(commentID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := m.client.LikeComment(ctx, commentID)
		if err != nil {
			return CommentsErrorMsg{Error: err}
		}
		// Reload comments to show updated like count
		return m.loadComments()()
	}
}

// View renders the view
func (m CommentsView) View() string {
	if !m.active {
		return ""
	}

	var sections []string

	// Title
	title := m.theme.Title.Render(fmt.Sprintf("💬 Comments: %s", m.mangaTitle))
	sections = append(sections, title)

	// Loading state
	if m.loading {
		loadingText := m.spinner.View() + " Loading comments..."
		sections = append(sections, loadingText)
		return lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	// Error display
	if m.lastError != nil {
		errorMsg := m.theme.ErrorText.Render(fmt.Sprintf("Error: %v", m.lastError))
		sections = append(sections, errorMsg)
		m.lastError = nil // Clear after display
	}

	// Comments count
	countStyle := m.theme.DimText
	countText := countStyle.Render(fmt.Sprintf("%d comments", len(m.comments)))
	sections = append(sections, countText)

	// Viewport with comments
	sections = append(sections, m.viewport.View())

	// Compose area
	if m.composing {
		composeLabel := m.theme.Primary.Bold(true).Render("▶ New Comment:")
		if m.posting {
			composeLabel += " " + m.spinner.View()
		}
		sections = append(sections, composeLabel)
		sections = append(sections, m.textarea.View())
		helpText := m.theme.DimText.Render("Ctrl+S: post | ESC: cancel")
		sections = append(sections, helpText)
	} else {
		// Help text
		helpText := m.theme.DimText.Render("↑/↓: navigate | c: new comment | l: like | r: refresh | q: back")
		sections = append(sections, helpText)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	containerStyle := lipgloss.NewStyle().
		Width(m.width-4).
		Padding(1, 2)

	return containerStyle.Render(content)
}

// renderCommentsList renders the list of comments
func (m CommentsView) renderCommentsList() string {
	if len(m.comments) == 0 {
		return m.theme.DimText.Render("No comments yet. Be the first to comment!")
	}

	var rows []string
	for i, comment := range m.comments {
		row := m.renderComment(comment, i == m.selectedIndex)
		rows = append(rows, row)
		if i < len(m.comments)-1 {
			rows = append(rows, m.theme.DimText.Render(strings.Repeat("─", 70)))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// renderComment renders a single comment
func (m CommentsView) renderComment(comment models.CommentWithReplies, selected bool) string {
	// Selector
	selector := "  "
	if selected {
		selector = m.theme.Primary.Render("▶ ")
	}

	// User and timestamp
	userStyle := m.theme.Primary.Bold(true)
	timeStyle := m.theme.DimText
	timeStr := formatTimestamp(comment.CreatedAt)

	header := selector + userStyle.Render(comment.CommentWithUser.Username) + " " + timeStyle.Render(timeStr)

	// Content
	contentStyle := m.theme.Description
	if selected {
		contentStyle = m.theme.Primary
	}
	content := contentStyle.Render(comment.Content)

	// Likes
	likesStyle := m.theme.DimText
	likes := likesStyle.Render(fmt.Sprintf("❤️  %d", comment.LikesCount))

	return lipgloss.JoinVertical(lipgloss.Left, header, content, likes, "")
}

// formatTimestamp formats a timestamp for display
func formatTimestamp(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%d min ago", mins)
	} else if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%d hours ago", hours)
	} else if diff < 7*24*time.Hour {
		days := int(diff.Hours() / 24)
		return fmt.Sprintf("%d days ago", days)
	}
	return t.Format("Jan 2")
}

// IsActive returns whether the view is active
func (m CommentsView) IsActive() bool {
	return m.active
}

// Close closes the view
func (m *CommentsView) Close() {
	m.active = false
}
