// Package styles - MangaHub Dracula Theme
// Hệ thống thiết kế TUI với màu Dracula
// Triết lý: "Bloomberg Terminal for Manga" - thông tin dày đặc nhưng sạch sẽ
//
// Color Palette (Dracula-inspired):
//   - Background: #282a36 (Dark Grey)
//   - Foreground: #f8f8f2 (White)
//   - Primary: #bd93f9 (Purple) - Focus/active states
//   - Secondary: #ff79c6 (Pink) - Highlights/hearts
//   - Success: #50fa7b (Green) - Progress bars/completed
//   - Warning: #ffb86c (Orange) - Ratings
//   - Dim: #6272a4 (Blue Grey) - Inactive text/borders
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// =====================================
// COLOR PALETTE - Dracula Theme Colors
// =====================================

var (
	// Base colors
	ColorBackground = lipgloss.Color("#282a36")
	ColorForeground = lipgloss.Color("#f8f8f2")

	// Accent colors
	ColorPrimary   = lipgloss.Color("#bd93f9") // Purple - focus/active
	ColorSecondary = lipgloss.Color("#ff79c6") // Pink - highlights
	ColorSuccess   = lipgloss.Color("#50fa7b") // Green - progress/completed
	ColorWarning   = lipgloss.Color("#ffb86c") // Orange - ratings
	ColorError     = lipgloss.Color("#ff5555") // Red - errors
	ColorCyan      = lipgloss.Color("#8be9fd") // Cyan - info

	// Utility colors
	ColorDim     = lipgloss.Color("#6272a4") // Blue Grey - inactive
	ColorComment = lipgloss.Color("#6272a4") // Same as dim
	ColorBlack   = lipgloss.Color("#21222c") // Darker background
)

// =====================================
// THEME STRUCT - Centralized Styling
// =====================================

// Theme contains all application styles
type Theme struct {
	// Base styles
	AppBox           lipgloss.Style
	Container        lipgloss.Style
	FocusedContainer lipgloss.Style

	// Header & Navigation
	Header       lipgloss.Style
	HeaderTitle  lipgloss.Style
	Tab          lipgloss.Style
	ActiveTab    lipgloss.Style
	InactiveTab  lipgloss.Style
	StatusOnline lipgloss.Style

	// Content styles
	Title       lipgloss.Style
	Subtitle    lipgloss.Style
	Description lipgloss.Style
	DimText     lipgloss.Style
	ErrorText   lipgloss.Style
	SuccessText lipgloss.Style

	// Interactive elements
	Button         lipgloss.Style
	ButtonActive   lipgloss.Style
	ButtonInactive lipgloss.Style
	Link           lipgloss.Style

	// List & Items
	ListItem         lipgloss.Style
	ListItemSelected lipgloss.Style
	ListItemDim      lipgloss.Style

	// Progress bar
	ProgressFull  lipgloss.Style
	ProgressEmpty lipgloss.Style
	ProgressText  lipgloss.Style

	// Rating
	RatingStar    lipgloss.Style
	RatingStarDim lipgloss.Style
	RatingNumber  lipgloss.Style

	// Cards & Panels
	Card        lipgloss.Style
	CardFocused lipgloss.Style
	Panel       lipgloss.Style
	PanelHeader lipgloss.Style

	// Activity Feed
	ActivityTime   lipgloss.Style
	ActivityUser   lipgloss.Style
	ActivityAction lipgloss.Style

	// Footer
	Footer     lipgloss.Style
	FooterKey  lipgloss.Style
	FooterText lipgloss.Style

	// Spinner
	Spinner lipgloss.Style

	// Direct Color Styles (convenience aliases)
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Success   lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
	Key       lipgloss.Style
	Badge     lipgloss.Style
}

// DefaultTheme is the global theme instance
var DefaultTheme = NewTheme()

// NewTheme creates a new Theme with Dracula colors
func NewTheme() *Theme {
	t := &Theme{}

	// ===== BASE STYLES =====

	// AppBox: Main application container
	t.AppBox = lipgloss.NewStyle().
		Background(ColorBackground).
		Foreground(ColorForeground).
		Padding(1)

	// Container: Generic bordered container
	t.Container = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDim).
		Padding(0, 1)

	// FocusedContainer: Container with focus highlight
	t.FocusedContainer = t.Container.
		BorderForeground(ColorPrimary)

	// ===== HEADER & NAVIGATION =====

	// Header: Top bar style
	t.Header = lipgloss.NewStyle().
		Bold(true).
		Background(ColorPrimary).
		Foreground(ColorBackground).
		Padding(0, 2).
		MarginBottom(1)

	// HeaderTitle: App title in header
	t.HeaderTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorForeground)

	// Tab: Inactive tab style
	t.Tab = lipgloss.NewStyle().
		Foreground(ColorDim).
		Padding(0, 2)

	// ActiveTab: Currently selected tab
	t.ActiveTab = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		Padding(0, 2).
		Border(lipgloss.Border{Bottom: "─"}).
		BorderForeground(ColorPrimary)

	// InactiveTab: Unselected tab
	t.InactiveTab = t.Tab

	// StatusOnline: Online indicator
	t.StatusOnline = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	// ===== CONTENT STYLES =====

	// Title: Main headings
	t.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary)

	// Subtitle: Secondary text
	t.Subtitle = lipgloss.NewStyle().
		Foreground(ColorCyan)

	// Description: Body text
	t.Description = lipgloss.NewStyle().
		Foreground(ColorForeground)

	// DimText: Muted/inactive text
	t.DimText = lipgloss.NewStyle().
		Foreground(ColorDim)

	// ErrorText: Error messages
	t.ErrorText = lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)

	// SuccessText: Success messages
	t.SuccessText = lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	// ===== INTERACTIVE ELEMENTS =====

	// Button: Clickable button
	t.Button = lipgloss.NewStyle().
		Foreground(ColorForeground).
		Background(ColorPrimary).
		Padding(0, 2).
		Bold(true)

	// ButtonActive: Focused button
	t.ButtonActive = t.Button.
		Background(ColorSecondary)

	// ButtonInactive: Disabled button
	t.ButtonInactive = lipgloss.NewStyle().
		Foreground(ColorDim).
		Background(ColorBlack).
		Padding(0, 2)

	// Link: Clickable link
	t.Link = lipgloss.NewStyle().
		Foreground(ColorCyan).
		Underline(true)

	// ===== LIST & ITEMS =====

	// ListItem: Normal list item
	t.ListItem = lipgloss.NewStyle().
		Foreground(ColorForeground).
		PaddingLeft(2)

	// ListItemSelected: Highlighted/selected item
	t.ListItemSelected = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		PaddingLeft(2).
		Background(ColorBlack)

	// ListItemDim: Inactive list item
	t.ListItemDim = lipgloss.NewStyle().
		Foreground(ColorDim).
		PaddingLeft(2)

	// ===== PROGRESS BAR =====

	// ProgressFull: Filled portion of progress bar
	t.ProgressFull = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	// ProgressEmpty: Empty portion of progress bar
	t.ProgressEmpty = lipgloss.NewStyle().
		Foreground(ColorDim)

	// ProgressText: Percentage text
	t.ProgressText = lipgloss.NewStyle().
		Foreground(ColorForeground).
		Bold(true)

	// ===== RATING =====

	// RatingStar: Filled star
	t.RatingStar = lipgloss.NewStyle().
		Foreground(ColorWarning)

	// RatingStarDim: Empty star
	t.RatingStarDim = lipgloss.NewStyle().
		Foreground(ColorDim)

	// RatingNumber: Numeric rating
	t.RatingNumber = lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true)

	// ===== CARDS & PANELS =====

	// Card: Content card
	t.Card = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDim).
		Padding(1, 2)

	// CardFocused: Focused card
	t.CardFocused = t.Card.
		BorderForeground(ColorPrimary)

	// Panel: Section panel
	t.Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorDim).
		Padding(0, 1)

	// PanelHeader: Panel title
	t.PanelHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	// ===== ACTIVITY FEED =====

	// ActivityTime: Timestamp
	t.ActivityTime = lipgloss.NewStyle().
		Foreground(ColorDim).
		Width(8)

	// ActivityUser: Username
	t.ActivityUser = lipgloss.NewStyle().
		Foreground(ColorCyan).
		Bold(true)

	// ActivityAction: Action description
	t.ActivityAction = lipgloss.NewStyle().
		Foreground(ColorForeground)

	// ===== FOOTER =====

	// Footer: Bottom bar
	t.Footer = lipgloss.NewStyle().
		Foreground(ColorDim).
		MarginTop(1).
		Padding(0, 1)

	// FooterKey: Keyboard shortcut highlight
	t.FooterKey = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	// FooterText: Footer description
	t.FooterText = lipgloss.NewStyle().
		Foreground(ColorDim)

	// ===== SPINNER =====

	t.Spinner = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	// ===== DIRECT COLOR STYLES (convenience) =====

	t.Primary = lipgloss.NewStyle().
		Foreground(ColorPrimary)

	t.Secondary = lipgloss.NewStyle().
		Foreground(ColorSecondary)

	t.Success = lipgloss.NewStyle().
		Foreground(ColorSuccess)

	t.Warning = lipgloss.NewStyle().
		Foreground(ColorWarning)

	t.Error = lipgloss.NewStyle().
		Foreground(ColorError)

	t.Key = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	t.Badge = lipgloss.NewStyle().
		Foreground(ColorBackground).
		Background(ColorPrimary).
		Padding(0, 1)

	return t
}

// =====================================
// HELPER FUNCTIONS - Rendering Utilities
// =====================================

// RenderProgressBar creates an ASCII progress bar
// percentage: 0.0 to 1.0
// width: total character width of bar
// Ví dụ: RenderProgressBar(0.75, 10) → "███████░░░ 75%"
func RenderProgressBar(percentage float64, width int) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	filled := int(percentage * float64(width))
	empty := width - filled

	bar := DefaultTheme.ProgressFull.Render(repeatChar("█", filled)) +
		DefaultTheme.ProgressEmpty.Render(repeatChar("░", empty))

	pct := DefaultTheme.ProgressText.Render(" " + formatPercent(percentage))

	return bar + pct
}

// RenderRating creates a star rating display
// rating: 0.0 to 5.0 (or 10.0 if scale10 is true)
// Ví dụ: RenderRating(4.5, false) → "★★★★☆"
func RenderRating(rating float64, scale10 bool) string {
	maxStars := 5
	if scale10 {
		rating = rating / 2.0 // Convert 10-scale to 5-scale
	}

	fullStars := int(rating)
	hasHalf := (rating - float64(fullStars)) >= 0.5
	emptyStars := maxStars - fullStars
	if hasHalf {
		emptyStars--
	}

	result := DefaultTheme.RatingStar.Render(repeatChar("★", fullStars))
	if hasHalf {
		result += DefaultTheme.RatingStar.Render("★") // Could use "⯪" for half
	}
	result += DefaultTheme.RatingStarDim.Render(repeatChar("☆", emptyStars))

	return result
}

// RenderRatingWithNumber shows rating with numeric value
// Ví dụ: RenderRatingWithNumber(9.2) → "⭐ 9.2"
func RenderRatingWithNumber(rating float64) string {
	return DefaultTheme.RatingStar.Render("⭐ ") +
		DefaultTheme.RatingNumber.Render(formatFloat(rating))
}

// RenderKeyHint creates a keyboard shortcut hint
// Ví dụ: RenderKeyHint("Enter", "Select") → "[Enter] Select"
func RenderKeyHint(key, action string) string {
	return DefaultTheme.FooterKey.Render("["+key+"]") + " " +
		DefaultTheme.FooterText.Render(action)
}

// RenderStatusBadge creates a status indicator
// Ví dụ: RenderStatusBadge("Reading", true) → "● Reading" (green if active)
func RenderStatusBadge(status string, active bool) string {
	if active {
		return DefaultTheme.SuccessText.Render("● " + status)
	}
	return DefaultTheme.DimText.Render("○ " + status)
}

// =====================================
// LAYOUT HELPERS - Responsive Design
// =====================================

// MinWidth returns minimum width for layout calculations
const (
	MinTerminalWidth  = 80
	MinTerminalHeight = 24
)

// PanelWidths calculates panel widths based on terminal width
// Returns (leftWidth, rightWidth) for split-pane layout
// Trả về chiều rộng cho layout 2 cột
func PanelWidths(totalWidth int) (int, int) {
	if totalWidth < MinTerminalWidth {
		// Stack vertically if too narrow
		return totalWidth - 4, 0
	}

	// 2:1 ratio for left:right
	padding := 4 // Account for borders
	usable := totalWidth - padding
	left := (usable * 2) / 3
	right := usable - left

	return left, right
}

// IsCompactMode checks if terminal is too narrow for side-by-side layout
func IsCompactMode(width int) bool {
	return width < MinTerminalWidth
}

// =====================================
// INTERNAL HELPERS
// =====================================

func repeatChar(char string, count int) string {
	if count <= 0 {
		return ""
	}
	result := ""
	for i := 0; i < count; i++ {
		result += char
	}
	return result
}

func formatPercent(p float64) string {
	return lipgloss.NewStyle().Render(
		string(rune('0'+int(p*100)/10)) +
			string(rune('0'+int(p*100)%10)) + "%")
}

func formatFloat(f float64) string {
	whole := int(f)
	frac := int((f - float64(whole)) * 10)
	return string(rune('0'+whole)) + "." + string(rune('0'+frac))
}

// =====================================
// ASCII ART - Decorative Elements
// =====================================

// MangaHubLogo returns the ASCII logo
func MangaHubLogo() string {
	return DefaultTheme.Title.Render(`
 ███╗   ███╗ █████╗ ███╗   ██╗ ██████╗  █████╗ 
 ████╗ ████║██╔══██╗████╗  ██║██╔════╝ ██╔══██╗
 ██╔████╔██║███████║██╔██╗ ██║██║  ███╗███████║
 ██║╚██╔╝██║██╔══██║██║╚██╗██║██║   ██║██╔══██║
 ██║ ╚═╝ ██║██║  ██║██║ ╚████║╚██████╔╝██║  ██║
 ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝ ╚═════╝ ╚═╝  ╚═╝
         ██╗  ██╗██╗   ██╗██████╗              
         ██║  ██║██║   ██║██╔══██╗             
         ███████║██║   ██║██████╔╝             
         ██╔══██║██║   ██║██╔══██╗             
         ██║  ██║╚██████╔╝██████╔╝             
         ╚═╝  ╚═╝ ╚═════╝ ╚═════╝              
`)
}

// BookIcon returns a simple book emoji/icon
func BookIcon() string {
	return "📚"
}

// FireIcon for trending
func FireIcon() string {
	return "🔥"
}

// StarIcon for ratings
func StarIcon() string {
	return "⭐"
}

// ActivityIcon for activity feed
func ActivityIcon() string {
	return "📌"
}
