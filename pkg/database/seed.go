package database

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"mangahub/pkg/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Seed populates the database with initial data
func (db *DB) Seed() error {
	// Check if already seeded
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM manga").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check seed status: %w", err)
	}

	if count > 0 {
		fmt.Println("Database already seeded, skipping...")
		return nil
	}

	fmt.Println("Seeding database...")

	// Seed admin user
	if err := db.seedAdminUser(); err != nil {
		return err
	}

	// Seed test users
	if err := db.seedTestUsers(); err != nil {
		return err
	}

	// Seed manga data
	if err := db.seedMangaData(); err != nil {
		return err
	}

	// Seed user activities
	if err := db.seedActivities(); err != nil {
		return err
	}

	// Seed reading statistics
	if err := db.seedReadingStats(); err != nil {
		return err
	}

	fmt.Println("Database seeded successfully!")
	return nil
}

func (db *DB) seedAdminUser() error {
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := models.User{
		ID:           uuid.New().String(),
		Username:     "admin",
		Email:        "admin@mangahub.com",
		PasswordHash: string(hash),
		DisplayName:  "Administrator",
		Bio:          "System Administrator",
		Role:         "admin",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, display_name, bio, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName,
		user.Bio, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
	)

	return err
}

func (db *DB) seedTestUsers() error {
	users := []struct {
		username string
		email    string
		display  string
	}{
		{"reader1", "reader1@example.com", "John Reader"},
		{"reader2", "reader2@example.com", "Jane Bookworm"},
		{"mangafan", "fan@example.com", "Manga Enthusiast"},
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	for _, u := range users {
		user := models.User{
			ID:           uuid.New().String(),
			Username:     u.username,
			Email:        u.email,
			PasswordHash: string(hash),
			DisplayName:  u.display,
			Bio:          "Test user account",
			Role:         "user",
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		_, err = db.Exec(`
			INSERT INTO users (id, username, email, password_hash, display_name, bio, role, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName,
			user.Bio, user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedMangaData() error {
	// Read manga.json if exists
	data, err := os.ReadFile("./data/manga.json")
	if err != nil {
		// If file doesn't exist, use default manga data
		return db.seedDefaultManga()
	}

	var mangaList []models.Manga
	if err := json.Unmarshal(data, &mangaList); err != nil {
		return fmt.Errorf("failed to parse manga.json: %w", err)
	}

	for _, manga := range mangaList {
		if manga.ID == "" {
			manga.ID = uuid.New().String()
		}
		manga.CreatedAt = time.Now()
		manga.UpdatedAt = time.Now()

		genresJSON, _ := json.Marshal(manga.Genres)
		_, err := db.Exec(`
			INSERT INTO manga (id, title, author, artist, description, cover_url, status, type, genres, total_chapters, rating, year, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			manga.ID, manga.Title, manga.Author, manga.Artist, manga.Description,
			manga.CoverURL, manga.Status, manga.Type, string(genresJSON),
			manga.TotalChapters, manga.Rating, manga.Year, manga.CreatedAt, manga.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedDefaultManga() error {
	defaultManga := []models.Manga{
		{
			ID:            uuid.New().String(),
			Title:         "One Piece",
			Author:        "Eiichiro Oda",
			Artist:        "Eiichiro Oda",
			Description:   "The story follows Monkey D. Luffy and his Straw Hat Pirates as they search for the ultimate treasure known as One Piece.",
			CoverURL:      "https://example.com/one-piece.jpg",
			Status:        "ongoing",
			Type:          "manga",
			Genres:        []string{"Action", "Adventure", "Fantasy"},
			TotalChapters: 1100,
			Rating:        9.2,
			Year:          1997,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            uuid.New().String(),
			Title:         "Attack on Titan",
			Author:        "Hajime Isayama",
			Artist:        "Hajime Isayama",
			Description:   "Humanity lives inside cities surrounded by three enormous walls that protect them from gigantic man-eating humanoids.",
			CoverURL:      "https://example.com/aot.jpg",
			Status:        "completed",
			Type:          "manga",
			Genres:        []string{"Action", "Drama", "Horror"},
			TotalChapters: 139,
			Rating:        8.9,
			Year:          2009,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            uuid.New().String(),
			Title:         "Solo Leveling",
			Author:        "Chugong",
			Artist:        "DUBU",
			Description:   "In a world where hunters fight monsters, the weakest hunter becomes the strongest through a mysterious leveling system.",
			CoverURL:      "https://example.com/solo-leveling.jpg",
			Status:        "completed",
			Type:          "manhwa",
			Genres:        []string{"Action", "Fantasy", "Adventure"},
			TotalChapters: 179,
			Rating:        9.1,
			Year:          2018,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		// Add more default manga...
	}

	for _, manga := range defaultManga {
		genresJSON, _ := json.Marshal(manga.Genres)
		_, err := db.Exec(`
			INSERT INTO manga (id, title, author, artist, description, cover_url, status, type, genres, total_chapters, rating, year, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			manga.ID, manga.Title, manga.Author, manga.Artist, manga.Description,
			manga.CoverURL, manga.Status, manga.Type, string(genresJSON),
			manga.TotalChapters, manga.Rating, manga.Year, manga.CreatedAt, manga.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// seedActivities generates sample activity feed entries
func (db *DB) seedActivities() error {
	// Get test users
	rows, err := db.Query("SELECT id, username FROM users WHERE role = 'user' LIMIT 3")
	if err != nil {
		return err
	}
	defer rows.Close()

	var users []struct {
		id       string
		username string
	}
	for rows.Next() {
		var u struct {
			id       string
			username string
		}
		if err := rows.Scan(&u.id, &u.username); err != nil {
			return err
		}
		users = append(users, u)
	}

	// Get manga
	mangaRows, err := db.Query("SELECT id, title FROM manga LIMIT 3")
	if err != nil {
		return err
	}
	defer mangaRows.Close()

	var mangaList []struct {
		id    string
		title string
	}
	for mangaRows.Next() {
		var m struct {
			id    string
			title string
		}
		if err := mangaRows.Scan(&m.id, &m.title); err != nil {
			return err
		}
		mangaList = append(mangaList, m)
	}

	if len(users) == 0 || len(mangaList) == 0 {
		return nil
	}

	// Generate activities for last 7 days
	activities := []struct {
		activityType string
		chapter      *int
		rating       *float64
	}{
		{"chapter_read", intPtr(5), nil},
		{"chapter_read", intPtr(10), nil},
		{"manga_rated", nil, float64Ptr(8.5)},
		{"chapter_read", intPtr(15), nil},
		{"manga_completed", nil, nil},
		{"manga_rated", nil, float64Ptr(9.0)},
		{"chapter_read", intPtr(1), nil},
		{"chapter_read", intPtr(20), nil},
	}

	for i, activity := range activities {
		user := users[i%len(users)]
		manga := mangaList[i%len(mangaList)]
		createdAt := time.Now().Add(-time.Duration(len(activities)-i) * 12 * time.Hour)

		_, err := db.Exec(`
			INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, chapter_number, rating, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), user.id, user.username, activity.activityType,
			manga.id, manga.title, activity.chapter, activity.rating, createdAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// seedReadingStats generates chapter history for statistics
func (db *DB) seedReadingStats() error {
	// Get test users
	rows, err := db.Query("SELECT id, username FROM users WHERE role = 'user' LIMIT 3")
	if err != nil {
		return err
	}
	defer rows.Close()

	var users []struct {
		id       string
		username string
	}
	for rows.Next() {
		var u struct {
			id       string
			username string
		}
		if err := rows.Scan(&u.id, &u.username); err != nil {
			return err
		}
		users = append(users, u)
	}

	// Get manga
	mangaRows, err := db.Query("SELECT id, title FROM manga LIMIT 3")
	if err != nil {
		return err
	}
	defer mangaRows.Close()

	var mangaList []struct {
		id    string
		title string
	}
	for mangaRows.Next() {
		var m struct {
			id    string
			title string
		}
		if err := mangaRows.Scan(&m.id, &m.title); err != nil {
			return err
		}
		mangaList = append(mangaList, m)
	}

	if len(users) == 0 || len(mangaList) == 0 {
		return nil
	}

	// Generate reading history for last 30 days
	now := time.Now()
	for daysAgo := 30; daysAgo >= 0; daysAgo-- {
		date := now.Add(-time.Duration(daysAgo) * 24 * time.Hour)

		// Random number of chapters read per day (0-5)
		chaptersPerDay := daysAgo % 6

		for i := 0; i < chaptersPerDay; i++ {
			user := users[i%len(users)]
			manga := mangaList[i%len(mangaList)]
			chapterNum := (30-daysAgo)*2 + i + 1 // Sequential chapters

			readTime := date.Add(time.Duration(i*2) * time.Hour)

			_, err := db.Exec(`
				INSERT INTO chapter_history (id, user_id, manga_id, manga_title, chapter_number, read_at)
				VALUES (?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), user.id, manga.id, manga.title, chapterNum, readTime,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}
