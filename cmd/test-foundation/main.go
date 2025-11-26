package main

import (
	"fmt"
	"log"

	"github.com/yourusername/mangahub/pkg/config"
	"github.com/yourusername/mangahub/pkg/database"
	"github.com/yourusername/mangahub/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load("./configs/development.yaml")
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Initialize logger
	logger.Init(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
		Output: cfg.Logging.Output,
	})

	logger.Info("Configuration loaded successfully")

	// Initialize database
	db, err := database.NewDB(database.Config{
		Path:            cfg.Database.Path,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		logger.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	logger.Info("Database initialized successfully")

	// Seed database
	if err := db.Seed(); err != nil {
		logger.Fatal("Failed to seed database:", err)
	}

	logger.Info("Database seeded successfully")

	// Verify data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	if err != nil {
		logger.Fatal("Failed to query manga:", err)
	}

	fmt.Printf("\n✅ Phase 1 Complete!\n")
	fmt.Printf("   - Configuration: Loaded\n")
	fmt.Printf("   - Database: Connected\n")
	fmt.Printf("   - Migrations: Applied\n")
	fmt.Printf("   - Seed Data: %d manga entries\n", count)
	fmt.Printf("\nReady for Phase 2: HTTP REST API\n\n")
}
