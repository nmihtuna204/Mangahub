// Package views - Statistics View
// Reading statistics page with charts and metrics
// Layout:
//
//	┌── 📊 Reading Statistics ───────────────────────────────┐
//	│ Total Manga: 47    │  Total Chapters: 8,432           │
//	│ Reading Streak: 23 days 🔥  │  Daily Average: 12 ch   │
//	├────────────────────────────────────────────────────────┤
//	│ Genre Distribution:     │  Monthly Activity:          │
//	│ █████████ Action 45%    │  Jan ████████ 120          │
//	│ █████ Romance 25%       │  Feb ██████ 85             │
//	└────────────────────────────────────────────────────────┘
package views

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"mangahub/internal/tui/api"
	"mangahub/internal/tui/styles"
	"mangahub/pkg/models"
)

// =====================================
// RANK SYSTEM
// =====================================

// Rank represents a user's reading rank
type Rank struct {
	Name        string
	Emoji       string
	Color       lipgloss.Color
	MinChapters int
	MaxChapters int // -1 for unlimited
}

// Ranks defines the ranking tiers based on chapters read
var Ranks = []Rank{
	{Name: "Bronze", Emoji: "🥉", Color: lipgloss.Color("#CD7F32"), MinChapters: 0, MaxChapters: 99},
	{Name: "Silver", Emoji: "🥈", Color: lipgloss.Color("#C0C0C0"), MinChapters: 100, MaxChapters: 499},
	{Name: "Gold", Emoji: "🥇", Color: lipgloss.Color("#FFD700"), MinChapters: 500, MaxChapters: 999},
	{Name: "Emerald", Emoji: "💎", Color: lipgloss.Color("#50C878"), MinChapters: 1000, MaxChapters: 2499},
	{Name: "Diamond", Emoji: "👑", Color: lipgloss.Color("#B9F2FF"), MinChapters: 2500, MaxChapters: -1},
}

// GetRank returns the rank for the given number of chapters read
func GetRank(chaptersRead int) Rank {
	for i := len(Ranks) - 1; i >= 0; i-- {
		if chaptersRead >= Ranks[i].MinChapters {
			return Ranks[i]
		}
	}
	return Ranks[0]
}

// GetNextRank returns the next rank tier (or nil if at max)
func GetNextRank(chaptersRead int) *Rank {
	currentRank := GetRank(chaptersRead)
	for i, r := range Ranks {
		if r.Name == currentRank.Name && i < len(Ranks)-1 {
			return &Ranks[i+1]
		}
	}
	return nil
}

// GetRankProgress returns progress to next rank (0.0 to 1.0)
func GetRankProgress(chaptersRead int) float64 {
	rank := GetRank(chaptersRead)
	if rank.MaxChapters == -1 {
		return 1.0 // Max rank reached
	}

	rangeSize := rank.MaxChapters - rank.MinChapters + 1
	progress := chaptersRead - rank.MinChapters
	return float64(progress) / float64(rangeSize)
}

// =====================================
// STATS MODEL
// =====================================

// StatsModel holds the statistics view state
type StatsModel struct {
	width  int
	height int
	theme  *styles.Theme

	// Data
	stats    *models.ReadingStats
	heatmap  []models.ReadingHeatmap
	overview *models.StatsOverview

	// Loading states
	loading   bool
	lastError error

	// Selection for keyboard nav
	selectedSection int // 0=overview, 1=genres, 2=monthly

	// Components
	spinner spinner.Model

	// API client
	client *api.Client
}

// NewStatsModel creates a new statistics model
func NewStatsModel(client *api.Client) *StatsModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(styles.ColorPrimary)

	return &StatsModel{
		theme:   styles.DefaultTheme,
		spinner: s,
		client:  client,
		loading: true,
	}
}

// NewStats creates a new stats model with default client
func NewStats() StatsModel {
	m := NewStatsModel(api.GetClient())
	return *m
}

// =====================================
// MESSAGES
// =====================================

type StatsLoadedMsg struct {
	Stats    *models.ReadingStats
	Overview *models.StatsOverview
	Heatmap  []models.ReadingHeatmap
}

type StatsErrorMsg struct {
	Error error
}

// =====================================
// COMMANDS
// =====================================

func (m StatsModel) loadStats() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Get comprehensive stats
		stats, err := m.client.GetStatistics(ctx)
		if err != nil {
			return StatsErrorMsg{Error: err}
		}

		// Get overview
		overview, err := m.client.GetStatsOverview(ctx)
		if err != nil {
			// Non-critical, continue with nil
			overview = nil
		}

		// Get heatmap
		heatmap, err := m.client.GetReadingHeatmap(ctx, 365)
		if err != nil {
			// Non-critical, continue with nil
			heatmap = nil
		}

		return StatsLoadedMsg{
			Stats:    stats,
			Overview: overview,
			Heatmap:  heatmap,
		}
	}
}

// =====================================
// MODEL METHODS
// =====================================

func (m StatsModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.loadStats(),
	)
}

func (m StatsModel) Update(msg tea.Msg) (StatsModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			// Refresh data
			m.loading = true
			return m, m.loadStats()
		case "j", "down":
			m.selectedSection = min(m.selectedSection+1, 2)
		case "k", "up":
			m.selectedSection = max(m.selectedSection-1, 0)
		}

	case StatsLoadedMsg:
		m.loading = false
		m.stats = msg.Stats
		m.overview = msg.Overview
		m.heatmap = msg.Heatmap
		m.lastError = nil

	case StatsErrorMsg:
		m.loading = false
		m.lastError = msg.Error

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m StatsModel) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	if m.loading {
		return m.renderLoading()
	}

	if m.lastError != nil {
		return m.renderError()
	}

	if m.stats == nil {
		return m.renderEmpty()
	}

	// Build sections
	sections := []string{
		m.renderHeader(),
		m.renderOverviewCards(),
		m.renderGenreDistribution(),
		m.renderMonthlyActivity(),
		m.renderAchievements(),
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return m.theme.Container.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(content)
}

// SetWidth sets the view width
func (m *StatsModel) SetWidth(w int) {
	m.width = w
}

// SetHeight sets the view height
func (m *StatsModel) SetHeight(h int) {
	m.height = h
}

// =====================================
// RENDER HELPERS
// =====================================

func (m StatsModel) renderLoading() string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		m.spinner.View(),
		m.theme.DimText.Render("Loading statistics..."),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m StatsModel) renderError() string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		m.theme.ErrorText.Render("⚠ Failed to load statistics"),
		m.theme.DimText.Render(m.lastError.Error()),
		"",
		m.theme.DimText.Render("[r] Retry"),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m StatsModel) renderEmpty() string {
	content := lipgloss.JoinVertical(lipgloss.Center,
		m.theme.Title.Render("📊 No Statistics Yet"),
		"",
		m.theme.DimText.Render("Start reading to build your stats!"),
		m.theme.DimText.Render("Your reading history and achievements will appear here."),
	)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m StatsModel) renderHeader() string {
	title := m.theme.Title.Render("📊 Reading Statistics")

	// Add rank badge
	rank := GetRank(m.stats.TotalChaptersRead)
	rankStyle := lipgloss.NewStyle().
		Foreground(rank.Color).
		Bold(true).
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(rank.Color)

	rankBadge := rankStyle.Render(fmt.Sprintf("%s %s Reader", rank.Emoji, rank.Name))

	subtitle := m.theme.DimText.Render(fmt.Sprintf("Last updated: %s", m.stats.UpdatedAt.Format("Jan 2, 2006 3:04 PM")))

	// Combine title and rank
	titleRow := lipgloss.JoinHorizontal(lipgloss.Center, title, "  ", rankBadge)

	return lipgloss.JoinVertical(lipgloss.Left, titleRow, subtitle, "")
}

func (m StatsModel) renderOverviewCards() string {
	cardWidth := (m.width - 10) / 4
	if cardWidth < 20 {
		cardWidth = 20
	}

	cardStyle := m.theme.Card.Width(cardWidth).Height(4).Align(lipgloss.Center)

	// Card 1: Total Manga
	card1 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Total Manga"),
		m.theme.Title.Render(fmt.Sprintf("%d", m.stats.TotalMangaRead)),
	))

	// Card 2: Total Chapters
	card2 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Chapters Read"),
		m.theme.Title.Render(formatNumber(m.stats.TotalChaptersRead)),
	))

	// Card 3: Reading Streak
	streakEmoji := "🔥"
	if m.stats.ReadingStreak < 3 {
		streakEmoji = "📖"
	}
	card3 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Current Streak"),
		m.theme.Success.Render(fmt.Sprintf("%d days %s", m.stats.ReadingStreak, streakEmoji)),
	))

	// Card 4: Daily Average
	card4 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Daily Average"),
		m.theme.Subtitle.Render(fmt.Sprintf("%.1f ch/day", m.stats.DailyAverage)),
	))

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, card1, " ", card2, " ", card3, " ", card4)

	// Second row
	// Card 5: Average Rating
	card5 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Avg Rating Given"),
		m.theme.Warning.Render(fmt.Sprintf("%.1f ★", m.stats.AverageRating)),
	))

	// Card 6: Reading Time
	hours := m.stats.TotalTimeMinutes / 60
	card6 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Total Time"),
		m.theme.Title.Render(fmt.Sprintf("%d hrs", hours)),
	))

	// Card 7: Most Read Genre
	card7 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Favorite Genre"),
		m.theme.Secondary.Render(m.stats.MostReadGenre),
	))

	// Card 8: Favorite Author
	author := m.stats.FavoriteAuthor
	if len(author) > 15 {
		author = author[:12] + "..."
	}
	card8 := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Center,
		m.theme.DimText.Render("Favorite Author"),
		m.theme.Subtitle.Render(author),
	))

	row2 := lipgloss.JoinHorizontal(lipgloss.Top, card5, " ", card6, " ", card7, " ", card8)

	return lipgloss.JoinVertical(lipgloss.Left, row1, "", row2, "", m.renderRankProgress())
}

func (m StatsModel) renderRankProgress() string {
	rank := GetRank(m.stats.TotalChaptersRead)
	nextRank := GetNextRank(m.stats.TotalChaptersRead)
	progress := GetRankProgress(m.stats.TotalChaptersRead)

	sectionTitle := m.theme.Subtitle.Render("🏆 Reading Rank")

	// Current rank display
	rankStyle := lipgloss.NewStyle().
		Foreground(rank.Color).
		Bold(true)
	currentRankText := rankStyle.Render(fmt.Sprintf("%s %s", rank.Emoji, rank.Name))

	// Progress bar
	barWidth := m.width - 30
	if barWidth < 20 {
		barWidth = 20
	}
	filledWidth := int(progress * float64(barWidth))
	emptyWidth := barWidth - filledWidth

	progressBar := lipgloss.NewStyle().Foreground(rank.Color).Render(strings.Repeat("█", filledWidth)) +
		m.theme.DimText.Render(strings.Repeat("░", emptyWidth))

	var progressLine string
	if nextRank != nil {
		chaptersNeeded := nextRank.MinChapters - m.stats.TotalChaptersRead
		nextStyle := lipgloss.NewStyle().Foreground(nextRank.Color).Bold(true)
		progressLine = fmt.Sprintf("%s → %s (%d chapters to go)",
			currentRankText,
			nextStyle.Render(fmt.Sprintf("%s %s", nextRank.Emoji, nextRank.Name)),
			chaptersNeeded,
		)
	} else {
		progressLine = fmt.Sprintf("%s - MAX RANK ACHIEVED! 👑", currentRankText)
	}

	percentText := m.theme.DimText.Render(fmt.Sprintf("%.0f%%", progress*100))
	barLine := lipgloss.JoinHorizontal(lipgloss.Center, progressBar, " ", percentText)

	// All ranks display
	var ranksDisplay []string
	for _, r := range Ranks {
		rStyle := lipgloss.NewStyle().Foreground(r.Color)
		if r.Name == rank.Name {
			rStyle = rStyle.Bold(true).Underline(true)
		}
		ranksDisplay = append(ranksDisplay, rStyle.Render(fmt.Sprintf("%s %s", r.Emoji, r.Name)))
	}
	allRanks := m.theme.DimText.Render("Ranks: ") + strings.Join(ranksDisplay, " → ")

	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle, progressLine, barLine, "", allRanks, "")
}

func (m StatsModel) renderGenreDistribution() string {
	if len(m.stats.GenreDistribution) == 0 {
		return ""
	}

	sectionTitle := m.theme.Subtitle.Render("📚 Genre Distribution")

	maxWidth := (m.width - 10) / 2
	barMaxWidth := maxWidth - 20

	var genres []string
	// Show top 5 genres
	displayCount := min(5, len(m.stats.GenreDistribution))

	for i := 0; i < displayCount; i++ {
		g := m.stats.GenreDistribution[i]
		barLen := int(g.Percentage / 100 * float64(barMaxWidth))
		if barLen < 1 && g.Percentage > 0 {
			barLen = 1
		}

		bar := m.theme.Success.Render(strings.Repeat("█", barLen))
		empty := m.theme.DimText.Render(strings.Repeat("░", barMaxWidth-barLen))

		line := fmt.Sprintf("%-12s %s%s %5.1f%%", truncateString(g.Genre, 12), bar, empty, g.Percentage)
		genres = append(genres, line)
	}

	genreContent := lipgloss.JoinVertical(lipgloss.Left, genres...)

	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle, genreContent, "")
}

func (m StatsModel) renderMonthlyActivity() string {
	if len(m.stats.MonthlyStats) == 0 {
		return ""
	}

	sectionTitle := m.theme.Subtitle.Render("📅 Monthly Activity")

	maxWidth := (m.width - 10) / 2
	barMaxWidth := maxWidth - 20

	// Find max for scaling
	maxChapters := 0
	for _, ms := range m.stats.MonthlyStats {
		if ms.ChaptersRead > maxChapters {
			maxChapters = ms.ChaptersRead
		}
	}

	var months []string
	displayCount := min(6, len(m.stats.MonthlyStats))

	for i := 0; i < displayCount; i++ {
		ms := m.stats.MonthlyStats[i]
		monthName := time.Month(ms.Month).String()[:3]

		barLen := 0
		if maxChapters > 0 {
			barLen = int(float64(ms.ChaptersRead) / float64(maxChapters) * float64(barMaxWidth))
		}
		if barLen < 1 && ms.ChaptersRead > 0 {
			barLen = 1
		}

		bar := m.theme.Primary.Render(strings.Repeat("█", barLen))
		empty := m.theme.DimText.Render(strings.Repeat("░", barMaxWidth-barLen))

		line := fmt.Sprintf("%s %d %s%s %4d ch", monthName, ms.Year%100, bar, empty, ms.ChaptersRead)
		months = append(months, line)
	}

	monthContent := lipgloss.JoinVertical(lipgloss.Left, months...)

	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle, monthContent, "")
}

func (m StatsModel) renderAchievements() string {
	sectionTitle := m.theme.Subtitle.Render("🏆 Records")

	var records []string

	if m.stats.FastestSeries != nil {
		fs := m.stats.FastestSeries
		records = append(records, fmt.Sprintf("⚡ Fastest: %s (%d days, %d chapters)",
			truncateString(fs.Title, 20), fs.Days, fs.Chapters))
	}

	if m.stats.SlowestSeries != nil {
		ss := m.stats.SlowestSeries
		records = append(records, fmt.Sprintf("🐢 Slowest: %s (%d days, %d chapters)",
			truncateString(ss.Title, 20), ss.Days, ss.Chapters))
	}

	if m.stats.LongestStreak > 0 {
		records = append(records, fmt.Sprintf("🔥 Longest Streak: %d days", m.stats.LongestStreak))
	}

	if len(records) == 0 {
		records = append(records, m.theme.DimText.Render("Keep reading to unlock records!"))
	}

	recordContent := lipgloss.JoinVertical(lipgloss.Left, records...)

	return lipgloss.JoinVertical(lipgloss.Left, sectionTitle, recordContent)
}

// =====================================
// UTILITY FUNCTIONS
// =====================================

func formatNumber(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// min and max are defined in other view files
