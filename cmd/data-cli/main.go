// Package main - MangaHub Data Pipeline TUI
// Interactive TUI for fetching external API data and importing to SQLite
//
// Features:
//   - Search MangaDex and Jikan APIs
//   - Preview data before import
//   - Import selected manga to local database
//   - Redis caching to save API calls
//   - Full pipeline testing
//
// Usage:
//
//	go run ./cmd/data-cli
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mangahub/pkg/cache"
	"mangahub/pkg/config"
	"mangahub/pkg/external"
	"mangahub/pkg/importer"
	"mangahub/pkg/models"

	_ "github.com/glebarez/go-sqlite"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ============================================================
// STYLES
// ============================================================

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B6B")).
			Background(lipgloss.Color("#1A1A2E")).
			Padding(0, 2)

	menuStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E94560")).
			Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00BFFF"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#E94560")).
			Padding(1, 2)
)

// ============================================================
// MODEL
// ============================================================

type appState int

const (
	stateMenu appState = iota
	stateSearch
	stateResults
	stateImportPreview
	stateImporting
	stateCacheMenu
	stateTopManga
	stateDBStats
)

type model struct {
	// State
	state       appState
	cursor      int
	selected    map[int]bool
	input       string
	inputMode   bool
	searchQuery string

	// Data
	searchResults   []models.ExternalMangaData
	topMangaList    []models.ExternalMangaData
	importPreviews  []importer.MangaPreview
	lastImportStats importer.ImportStats
	dbStats         dbStatistics

	// Services
	cfg            *config.Config
	db             *sql.DB
	redisCache     *cache.RedisCache
	mangadexClient *external.MangaDexClient
	jikanClient    *external.JikanClient
	dataImporter   *importer.Importer

	// Status
	statusMsg    string
	errorMsg     string
	isLoading    bool
	searchSource string // "mangadex" or "jikan"

	// Terminal size
	width  int
	height int
}

type dbStatistics struct {
	MangaCount    int
	UserCount     int
	ProgressCount int
	RatingsCount  int
	CacheKeys     int
}

// Menu items
var menuItems = []string{
	"🔍 Search MangaDex",
	"🔍 Search Jikan/MAL",
	"🏆 Import Top Manga (MAL)",
	"📦 View Cache Status",
	"📊 Database Statistics",
	"🧪 Run Pipeline Test",
	"❌ Exit",
}

func initialModel() model {
	return model{
		state:    stateMenu,
		cursor:   0,
		selected: make(map[int]bool),
		width:    80,
		height:   24,
	}
}

// ============================================================
// MESSAGES
// ============================================================

type initMsg struct {
	cfg      *config.Config
	db       *sql.DB
	cache    *cache.RedisCache
	mangadex *external.MangaDexClient
	jikan    *external.JikanClient
	imp      *importer.Importer
	err      error
}

type searchResultsMsg struct {
	results []models.ExternalMangaData
	err     error
}

type topMangaMsg struct {
	results []models.ExternalMangaData
	err     error
}

type importDoneMsg struct {
	stats importer.ImportStats
	err   error
}

type dbStatsMsg struct {
	stats dbStatistics
	err   error
}

type cacheStatsMsg struct {
	keys int
	err  error
}

// ============================================================
// INIT & UPDATE
// ============================================================

func (m model) Init() tea.Cmd {
	return initializeApp
}

func initializeApp() tea.Msg {
	// Load config
	cfg, err := config.Load("./configs/development.yaml")
	if err != nil {
		cfg = &config.Config{}
		setDefaults(cfg)
	}

	// Initialize database
	dbPath := filepath.Join(".", "data", "mangahub.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		return initMsg{err: fmt.Errorf("database error: %w", err)}
	}

	// Initialize clients
	mangadex := external.NewMangaDexClient(&cfg.MangaDex)
	jikan := external.NewJikanClient(&cfg.Jikan)

	// Initialize cache (optional)
	var redisCache *cache.RedisCache
	redisCache, _ = cache.NewRedisCache(&cfg.Redis)

	// Initialize importer
	imp := importer.NewImporter(db, redisCache)

	return initMsg{
		cfg:      cfg,
		db:       db,
		cache:    redisCache,
		mangadex: mangadex,
		jikan:    jikan,
		imp:      imp,
	}
}

func setDefaults(cfg *config.Config) {
	cfg.MangaDex.BaseURL = "https://api.mangadex.org"
	cfg.MangaDex.RateLimit = 5
	cfg.MangaDex.Timeout = 30 * time.Second
	cfg.Jikan.BaseURL = "https://api.jikan.moe/v4"
	cfg.Jikan.RateLimit = 3
	cfg.Jikan.Timeout = 30 * time.Second
	cfg.Redis.Host = "localhost"
	cfg.Redis.Port = 6379
	cfg.Redis.PoolSize = 10
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case initMsg:
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		m.cfg = msg.cfg
		m.db = msg.db
		m.redisCache = msg.cache
		m.mangadexClient = msg.mangadex
		m.jikanClient = msg.jikan
		m.dataImporter = msg.imp
		m.statusMsg = "✅ All services initialized"
		return m, nil

	case searchResultsMsg:
		m.isLoading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		m.searchResults = msg.results
		m.selected = make(map[int]bool)
		m.cursor = 0
		m.state = stateResults
		m.statusMsg = fmt.Sprintf("Found %d results", len(msg.results))
		return m, nil

	case topMangaMsg:
		m.isLoading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		m.topMangaList = msg.results
		m.searchResults = msg.results
		m.selected = make(map[int]bool)
		m.cursor = 0
		m.state = stateResults
		m.statusMsg = fmt.Sprintf("Loaded top %d manga", len(msg.results))
		return m, nil

	case importDoneMsg:
		m.isLoading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		m.lastImportStats = msg.stats
		m.statusMsg = fmt.Sprintf("✅ Imported: %d new, %d updated, %d failed",
			msg.stats.Inserted, msg.stats.Updated, msg.stats.Failed)
		m.state = stateMenu
		return m, nil

	case dbStatsMsg:
		m.isLoading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return m, nil
		}
		m.dbStats = msg.stats
		m.state = stateDBStats
		return m, nil

	case cacheStatsMsg:
		m.isLoading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
		} else {
			m.dbStats.CacheKeys = msg.keys
		}
		return m, nil
	}

	return m, nil
}

func (m model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "q":
		if m.state == stateMenu && !m.inputMode {
			return m, tea.Quit
		}
		if !m.inputMode {
			m.state = stateMenu
			m.errorMsg = ""
			return m, nil
		}

	case "esc":
		if m.inputMode {
			m.inputMode = false
			m.input = ""
			m.state = stateMenu
			return m, nil
		}
		if m.state != stateMenu {
			m.state = stateMenu
			m.errorMsg = ""
			return m, nil
		}
	}

	// Input mode
	if m.inputMode {
		switch msg.String() {
		case "enter":
			m.inputMode = false
			m.searchQuery = m.input
			m.input = ""
			m.isLoading = true
			return m, m.performSearch()
		case "backspace":
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
		default:
			if len(msg.String()) == 1 {
				m.input += msg.String()
			}
		}
		return m, nil
	}

	// State-specific handling
	switch m.state {
	case stateMenu:
		return m.handleMenuKeys(msg)
	case stateResults:
		return m.handleResultsKeys(msg)
	case stateDBStats, stateCacheMenu:
		return m.handleStatsKeys(msg)
	}

	return m, nil
}

func (m model) handleMenuKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(menuItems)-1 {
			m.cursor++
		}
	case "enter":
		return m.selectMenuItem()
	}
	return m, nil
}

func (m model) selectMenuItem() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0: // Search MangaDex
		m.searchSource = "mangadex"
		m.inputMode = true
		m.state = stateSearch
		m.input = ""
	case 1: // Search Jikan
		m.searchSource = "jikan"
		m.inputMode = true
		m.state = stateSearch
		m.input = ""
	case 2: // Import Top Manga
		m.isLoading = true
		m.statusMsg = "Fetching top manga from MAL..."
		return m, m.fetchTopManga()
	case 3: // Cache Status
		m.state = stateCacheMenu
		return m, m.fetchCacheStats()
	case 4: // DB Statistics
		m.isLoading = true
		return m, m.fetchDBStats()
	case 5: // Pipeline Test
		m.statusMsg = "Running pipeline test..."
		m.isLoading = true
		return m, m.runPipelineTest()
	case 6: // Exit
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handleResultsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.searchResults)-1 {
			m.cursor++
		}
	case " ":
		// Toggle selection
		m.selected[m.cursor] = !m.selected[m.cursor]
	case "a":
		// Select all
		for i := range m.searchResults {
			m.selected[i] = true
		}
	case "n":
		// Deselect all
		m.selected = make(map[int]bool)
	case "i":
		// Import selected
		if len(m.selected) == 0 {
			m.errorMsg = "No items selected. Press SPACE to select."
			return m, nil
		}
		m.isLoading = true
		m.statusMsg = "Importing selected manga..."
		return m, m.importSelected()
	case "I":
		// Import all
		for i := range m.searchResults {
			m.selected[i] = true
		}
		m.isLoading = true
		m.statusMsg = "Importing all results..."
		return m, m.importSelected()
	}
	return m, nil
}

func (m model) handleStatsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.isLoading = true
		return m, m.fetchDBStats()
	}
	return m, nil
}

// ============================================================
// COMMANDS
// ============================================================

func (m model) performSearch() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var results []models.ExternalMangaData
		var err error

		// Check cache first
		cacheKey := cache.BuildKey(cache.PrefixSearch, m.searchSource+":"+m.searchQuery)
		if m.redisCache != nil {
			cached, _ := m.redisCache.Get(ctx, cacheKey)
			if cached != "" {
				if err := json.Unmarshal([]byte(cached), &results); err == nil && len(results) > 0 {
					return searchResultsMsg{results: results}
				}
			}
		}

		// Fetch from API
		if m.searchSource == "mangadex" {
			results, err = m.mangadexClient.SearchMangaFiltered(ctx, m.searchQuery, 10, 0)
		} else {
			results, err = m.jikanClient.SearchMangaFiltered(ctx, m.searchQuery, 1, 10)
		}

		if err != nil {
			return searchResultsMsg{err: err}
		}

		// Cache results
		if m.redisCache != nil && len(results) > 0 {
			m.redisCache.Set(ctx, cacheKey, results, cache.TTLLong)
		}

		return searchResultsMsg{results: results}
	}
}

func (m model) fetchTopManga() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Check cache
		cacheKey := cache.BuildKey(cache.PrefixExternal, "jikan:top:25")
		if m.redisCache != nil {
			cached, _ := m.redisCache.Get(ctx, cacheKey)
			if cached != "" {
				var results []models.ExternalMangaData
				if err := json.Unmarshal([]byte(cached), &results); err == nil && len(results) > 0 {
					return topMangaMsg{results: results}
				}
			}
		}

		// Fetch from Jikan
		resp, err := m.jikanClient.GetTopManga(ctx, 1, 25, "")
		if err != nil {
			return topMangaMsg{err: err}
		}

		results := make([]models.ExternalMangaData, 0, len(resp.Data))
		for _, item := range resp.Data {
			results = append(results, item.ToExternalMangaData())
		}

		// Cache results
		if m.redisCache != nil && len(results) > 0 {
			m.redisCache.Set(ctx, cacheKey, results, cache.TTLLong)
		}

		return topMangaMsg{results: results}
	}
}

func (m model) importSelected() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Collect selected items
		toImport := make([]models.ExternalMangaData, 0)
		for i, selected := range m.selected {
			if selected && i < len(m.searchResults) {
				toImport = append(toImport, m.searchResults[i])
			}
		}

		if len(toImport) == 0 {
			return importDoneMsg{err: fmt.Errorf("no items to import")}
		}

		// Reset importer stats
		m.dataImporter.ResetStats()

		// Import batch
		_, err := m.dataImporter.ImportBatch(ctx, toImport)
		if err != nil {
			return importDoneMsg{err: err}
		}

		return importDoneMsg{stats: m.dataImporter.GetStats()}
	}
}

func (m model) fetchDBStats() tea.Cmd {
	return func() tea.Msg {
		var stats dbStatistics

		// Count manga
		m.db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&stats.MangaCount)
		m.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.UserCount)
		m.db.QueryRow("SELECT COUNT(*) FROM reading_progress").Scan(&stats.ProgressCount)
		m.db.QueryRow("SELECT COUNT(*) FROM manga_ratings").Scan(&stats.RatingsCount)

		return dbStatsMsg{stats: stats}
	}
}

func (m model) fetchCacheStats() tea.Cmd {
	return func() tea.Msg {
		if m.redisCache == nil {
			return cacheStatsMsg{keys: 0, err: fmt.Errorf("Redis not connected")}
		}

		ctx := context.Background()
		if err := m.redisCache.Ping(ctx); err != nil {
			return cacheStatsMsg{err: err}
		}

		return cacheStatsMsg{keys: -1}
	}
}

func (m model) runPipelineTest() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Test 1: MangaDex search
		results, err := m.mangadexClient.SearchMangaFiltered(ctx, "one piece", 2, 0)
		if err != nil {
			return searchResultsMsg{err: fmt.Errorf("MangaDex test failed: %w", err)}
		}

		if len(results) == 0 {
			return searchResultsMsg{err: fmt.Errorf("MangaDex returned no results")}
		}

		// Test 2: Import one result
		m.dataImporter.ResetStats()
		_, err = m.dataImporter.ImportOne(ctx, results[0])
		if err != nil {
			return importDoneMsg{err: fmt.Errorf("Import test failed: %w", err)}
		}

		return importDoneMsg{stats: m.dataImporter.GetStats()}
	}
}

// ============================================================
// VIEW
// ============================================================

func (m model) View() string {
	var s strings.Builder

	// Header
	s.WriteString(titleStyle.Render("  🗃️  MangaHub Data Pipeline TUI  "))
	s.WriteString("\n\n")

	// Main content based on state
	switch m.state {
	case stateMenu:
		s.WriteString(m.viewMenu())
	case stateSearch:
		s.WriteString(m.viewSearch())
	case stateResults:
		s.WriteString(m.viewResults())
	case stateDBStats:
		s.WriteString(m.viewDBStats())
	case stateCacheMenu:
		s.WriteString(m.viewCacheStatus())
	}

	// Status bar
	s.WriteString("\n")
	if m.isLoading {
		s.WriteString(infoStyle.Render("⏳ Loading..."))
	} else if m.errorMsg != "" {
		s.WriteString(errorStyle.Render("❌ " + m.errorMsg))
	} else if m.statusMsg != "" {
		s.WriteString(successStyle.Render(m.statusMsg))
	}

	// Help
	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render(m.getHelpText()))

	return s.String()
}

func (m model) viewMenu() string {
	var s strings.Builder
	s.WriteString(menuStyle.Render("Main Menu"))
	s.WriteString("\n\n")

	for i, item := range menuItems {
		cursor := "  "
		style := dimStyle
		if i == m.cursor {
			cursor = "▶ "
			style = selectedStyle
		}
		s.WriteString(cursor + style.Render(item) + "\n")
	}

	return s.String()
}

func (m model) viewSearch() string {
	var s strings.Builder
	source := "MangaDex"
	if m.searchSource == "jikan" {
		source = "Jikan/MAL"
	}

	s.WriteString(menuStyle.Render(fmt.Sprintf("Search %s", source)))
	s.WriteString("\n\n")
	s.WriteString("Enter search query:\n\n")
	s.WriteString(boxStyle.Render(m.input + "▌"))
	s.WriteString("\n\n")
	s.WriteString(dimStyle.Render("Press ENTER to search, ESC to cancel"))

	return s.String()
}

func (m model) viewResults() string {
	var s strings.Builder
	s.WriteString(menuStyle.Render(fmt.Sprintf("Search Results (%d found)", len(m.searchResults))))
	s.WriteString("\n\n")

	if len(m.searchResults) == 0 {
		s.WriteString(dimStyle.Render("No results found."))
		return s.String()
	}

	// Count selected
	selectedCount := 0
	for _, sel := range m.selected {
		if sel {
			selectedCount++
		}
	}
	if selectedCount > 0 {
		s.WriteString(infoStyle.Render(fmt.Sprintf("Selected: %d items", selectedCount)))
		s.WriteString("\n\n")
	}

	// Show results with scrolling
	visibleCount := 8
	start := 0
	if m.cursor >= visibleCount {
		start = m.cursor - visibleCount + 1
	}
	end := start + visibleCount
	if end > len(m.searchResults) {
		end = len(m.searchResults)
	}

	for i := start; i < end; i++ {
		result := m.searchResults[i]
		cursor := "  "
		checkbox := "[ ]"
		style := dimStyle

		if i == m.cursor {
			cursor = "▶ "
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
		}
		if m.selected[i] {
			checkbox = "[✓]"
		}

		// Truncate title
		title := result.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}

		rating := "N/A"
		if result.Rating > 0 {
			rating = fmt.Sprintf("%.1f", result.Rating)
		}

		line := fmt.Sprintf("%s %s %-40s │ %s │ %s",
			cursor, checkbox, title, rating, result.Source)
		s.WriteString(style.Render(line) + "\n")
	}

	if len(m.searchResults) > visibleCount {
		s.WriteString(dimStyle.Render(fmt.Sprintf("\n... showing %d-%d of %d", start+1, end, len(m.searchResults))))
	}

	return s.String()
}

func (m model) viewDBStats() string {
	var s strings.Builder
	s.WriteString(menuStyle.Render("📊 Database Statistics"))
	s.WriteString("\n\n")

	stats := []string{
		fmt.Sprintf("  📚 Manga entries:     %d", m.dbStats.MangaCount),
		fmt.Sprintf("  👤 Users:             %d", m.dbStats.UserCount),
		fmt.Sprintf("  📖 Reading progress:  %d", m.dbStats.ProgressCount),
		fmt.Sprintf("  ⭐ Ratings:           %d", m.dbStats.RatingsCount),
	}

	if m.redisCache != nil {
		stats = append(stats, "  🗄️  Redis:            Connected")
	} else {
		stats = append(stats, "  🗄️  Redis:            Not connected")
	}

	s.WriteString(boxStyle.Render(strings.Join(stats, "\n")))

	return s.String()
}

func (m model) viewCacheStatus() string {
	var s strings.Builder
	s.WriteString(menuStyle.Render("📦 Cache Status"))
	s.WriteString("\n\n")

	if m.redisCache == nil {
		s.WriteString(errorStyle.Render("Redis is not connected.\n\n"))
		s.WriteString(dimStyle.Render("To start Redis:\n"))
		s.WriteString(dimStyle.Render("  docker run -d --name mangahub-redis -p 6379:6379 redis:7-alpine"))
	} else {
		s.WriteString(successStyle.Render("✅ Redis connected\n"))
		s.WriteString(dimStyle.Render(fmt.Sprintf("Host: %s:%d", m.cfg.Redis.Host, m.cfg.Redis.Port)))
	}

	return s.String()
}

func (m model) getHelpText() string {
	switch m.state {
	case stateMenu:
		return "↑/↓: Navigate • Enter: Select • q: Quit"
	case stateResults:
		return "↑/↓: Navigate • SPACE: Toggle • a: All • n: None • i: Import selected • I: Import all • ESC: Back"
	case stateDBStats:
		return "r: Refresh • ESC: Back"
	default:
		return "ESC: Back • q: Quit"
	}
}

// ============================================================
// MAIN
// ============================================================

func main() {
	// Check for CLI mode
	if len(os.Args) > 1 {
		runCLIMode(os.Args)
		return
	}

	// Run TUI mode
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}

// ============================================================
// CLI MODE (for non-interactive usage)
// ============================================================

func runCLIMode(args []string) {
	if len(args) < 2 {
		printCLIHelp()
		return
	}

	// Load config
	cfg, err := config.Load("./configs/development.yaml")
	if err != nil {
		cfg = &config.Config{}
		setDefaults(cfg)
	}

	// Initialize database
	dbPath := filepath.Join(".", "data", "mangahub.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		fmt.Printf("❌ Database error: %v\n", err)
		return
	}
	defer db.Close()

	// Initialize clients
	mangadex := external.NewMangaDexClient(&cfg.MangaDex)
	jikan := external.NewJikanClient(&cfg.Jikan)
	redisCache, _ := cache.NewRedisCache(&cfg.Redis)
	imp := importer.NewImporter(db, redisCache)

	ctx := context.Background()
	cmd := args[1]

	switch cmd {
	case "search", "searchj", "sj":
		// Use Jikan (more reliable) for searchj/sj, MangaDex for search
		useJikan := cmd == "searchj" || cmd == "sj"
		if len(args) < 3 {
			fmt.Println("Usage: data-cli search <query>")
			fmt.Println("       data-cli searchj <query>  (use Jikan/MAL)")
			return
		}
		query := strings.Join(args[2:], " ")

		var results []models.ExternalMangaData
		var err error

		if useJikan {
			fmt.Printf("🔍 Searching Jikan/MAL for: %s\n", query)
			results, err = jikan.SearchMangaFiltered(ctx, query, 1, 10)
		} else {
			fmt.Printf("🔍 Searching MangaDex for: %s\n", query)
			results, err = mangadex.SearchMangaFiltered(ctx, query, 10, 0)
		}

		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}

		fmt.Printf("\n📚 Found %d results:\n", len(results))
		for i, r := range results {
			rating := "N/A"
			if r.Rating > 0 {
				rating = fmt.Sprintf("%.2f", r.Rating)
			}
			fmt.Printf("%d. %s (Rating: %s, %s)\n", i+1, r.Title, rating, r.Source)
		}

	case "import", "importj", "ij":
		// Use Jikan for importj/ij, MangaDex for import
		useJikan := cmd == "importj" || cmd == "ij"
		if len(args) < 3 {
			fmt.Println("Usage: data-cli import <query>")
			fmt.Println("       data-cli importj <query>  (use Jikan/MAL)")
			return
		}
		query := strings.Join(args[2:], " ")

		var results []models.ExternalMangaData
		var err error

		if useJikan {
			fmt.Printf("🔍 Searching Jikan/MAL for: %s\n", query)
			results, err = jikan.SearchMangaFiltered(ctx, query, 1, 10)
		} else {
			fmt.Printf("🔍 Searching MangaDex for: %s\n", query)
			results, err = mangadex.SearchMangaFiltered(ctx, query, 10, 0)
		}

		if err != nil {
			fmt.Printf("❌ Search error: %v\n", err)
			return
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			return
		}

		fmt.Printf("📥 Importing %d manga...\n", len(results))
		_, err = imp.ImportBatch(ctx, results)
		if err != nil {
			fmt.Printf("❌ Import error: %v\n", err)
			return
		}

		stats := imp.GetStats()
		fmt.Printf("✅ Done! Inserted: %d, Updated: %d, Failed: %d\n",
			stats.Inserted, stats.Updated, stats.Failed)

	case "top":
		count := 25
		if len(args) >= 3 {
			if n, err := strconv.Atoi(args[2]); err == nil {
				count = n
			}
		}
		fmt.Printf("🏆 Fetching top %d manga from MAL...\n", count)

		resp, err := jikan.GetTopManga(ctx, 1, count, "")
		if err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			return
		}

		results := make([]models.ExternalMangaData, 0, len(resp.Data))
		for _, item := range resp.Data {
			results = append(results, item.ToExternalMangaData())
		}

		fmt.Printf("📥 Importing %d manga...\n", len(results))
		_, err = imp.ImportBatch(ctx, results)
		if err != nil {
			fmt.Printf("❌ Import error: %v\n", err)
			return
		}

		stats := imp.GetStats()
		fmt.Printf("✅ Done! Inserted: %d, Updated: %d, Failed: %d\n",
			stats.Inserted, stats.Updated, stats.Failed)

	case "stats":
		fmt.Println("📊 Database Statistics")
		fmt.Println("─────────────────────")

		var count int
		db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
		fmt.Printf("  📚 Manga:    %d\n", count)

		db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		fmt.Printf("  👤 Users:    %d\n", count)

		db.QueryRow("SELECT COUNT(*) FROM reading_progress").Scan(&count)
		fmt.Printf("  📖 Progress: %d\n", count)

		db.QueryRow("SELECT COUNT(*) FROM manga_ratings").Scan(&count)
		fmt.Printf("  ⭐ Ratings:  %d\n", count)

		if redisCache != nil {
			fmt.Printf("  🗄️  Redis:   Connected\n")
		} else {
			fmt.Printf("  🗄️  Redis:   Not connected\n")
		}

	default:
		fmt.Printf("Unknown command: %s\n", cmd)
		printCLIHelp()
	}
}

func printCLIHelp() {
	fmt.Println("MangaHub Data Pipeline CLI")
	fmt.Println()
	fmt.Println("Usage: data-cli [command] [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  (no args)        Launch interactive TUI")
	fmt.Println("  search <query>   Search MangaDex")
	fmt.Println("  searchj <query>  Search Jikan/MAL (recommended)")
	fmt.Println("  import <query>   Search MangaDex and import to database")
	fmt.Println("  importj <query>  Search Jikan/MAL and import (recommended)")
	fmt.Println("  top [count]      Import top manga from MAL (default: 25)")
	fmt.Println("  stats            Show database statistics")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  data-cli                     # Launch TUI")
	fmt.Println("  data-cli searchj \"one piece\" # Search Jikan")
	fmt.Println("  data-cli importj naruto      # Import from Jikan")
	fmt.Println("  data-cli top 50              # Import top 50")
}
