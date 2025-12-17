// Package main - MangaHub Terminal User Interface
// Bloomberg Terminal-inspired manga reading tracker
// Entry point for the TUI application
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"mangahub/internal/tui"
	"mangahub/internal/tui/api"
	"mangahub/pkg/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load("configs/development.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize API client
	baseURL := fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port)
	api.InitClient(baseURL)

	// Create the TUI application
	app := tui.NewApp()

	// Configure Bubble Tea program
	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
