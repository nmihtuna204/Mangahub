package database

import (
	"fmt"
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

	// Seed genres
	if err := db.seedGenres(); err != nil {
		return err
	}

	// Seed manga data with 10 samples
	if err := db.seedMangaData(); err != nil {
		return err
	}

	// Seed user reading progress
	if err := db.seedReadingProgress(); err != nil {
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
		Role:         "admin",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = db.Exec(`
		INSERT INTO users (id, username, email, password_hash, display_name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName,
		user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
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
			Role:         "user",
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		_, err = db.Exec(`
			INSERT INTO users (id, username, email, password_hash, display_name, role, is_active, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			user.ID, user.Username, user.Email, user.PasswordHash, user.DisplayName,
			user.Role, user.IsActive, user.CreatedAt, user.UpdatedAt,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedGenres() error {
	genres := []struct {
		name string
		slug string
	}{
		{"Action", "action"},
		{"Adventure", "adventure"},
		{"Comedy", "comedy"},
		{"Drama", "drama"},
		{"Fantasy", "fantasy"},
		{"Horror", "horror"},
		{"Isekai", "isekai"},
		{"Mecha", "mecha"},
		{"Mystery", "mystery"},
		{"Romance", "romance"},
		{"Sci-Fi", "sci-fi"},
		{"Slice of Life", "slice-of-life"},
		{"Sports", "sports"},
		{"Supernatural", "supernatural"},
		{"Thriller", "thriller"},
	}

	for _, g := range genres {
		_, err := db.Exec(`
			INSERT INTO genres (id, name, slug, created_at)
			VALUES (?, ?, ?, ?)`,
			uuid.New().String(), g.name, g.slug, time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedMangaData() error {
	// 10 sample manga entries matching new normalized schema
	mangaList := []struct {
		title       string
		author      string
		artist      string
		description string
		status      string
		mangaType   string
		year        int
		chapters    int
		genres      []string
	}{
		{
			title:       "One Piece",
			author:      "Eiichiro Oda",
			artist:      "Eiichiro Oda",
			description: "Follow Monkey D. Luffy and his Straw Hat Pirates as they search for the ultimate treasure, One Piece.",
			status:      "ongoing",
			mangaType:   "manga",
			year:        1997,
			chapters:    1100,
			genres:      []string{"Action", "Adventure", "Fantasy"},
		},
		{
			title:       "Attack on Titan",
			author:      "Hajime Isayama",
			artist:      "Hajime Isayama",
			description: "Humanity lives inside cities surrounded by massive walls protecting them from gigantic humanoid creatures.",
			status:      "completed",
			mangaType:   "manga",
			year:        2009,
			chapters:    139,
			genres:      []string{"Action", "Drama", "Horror"},
		},
		{
			title:       "Solo Leveling",
			author:      "Chugong",
			artist:      "DUBU",
			description: "In a world where hunters fight monsters, the weakest hunter becomes the strongest through a mysterious system.",
			status:      "completed",
			mangaType:   "manhwa",
			year:        2018,
			chapters:    179,
			genres:      []string{"Action", "Fantasy", "Adventure"},
		},
		{
			title:       "Demon Slayer",
			author:      "Koyoharu Gotouge",
			artist:      "Koyoharu Gotouge",
			description: "Tanjiro's journey to save his sister from demons while becoming a powerful demon slayer.",
			status:      "completed",
			mangaType:   "manga",
			year:        2018,
			chapters:    205,
			genres:      []string{"Action", "Adventure", "Supernatural"},
		},
		{
			title:       "My Hero Academia",
			author:      "Kohei Horikoshi",
			artist:      "Kohei Horikoshi",
			description: "In a world where most people have superpowers called Quirks, a powerless boy dreams of becoming a hero.",
			status:      "ongoing",
			mangaType:   "manga",
			year:        2014,
			chapters:    426,
			genres:      []string{"Action", "Adventure", "Comedy"},
		},
		{
			title:       "Steins;Gate",
			author:      "Anonymous",
			artist:      "Hiyama Mizuho",
			description: "A group discovers how to send messages to the past and must prevent a dystopian future.",
			status:      "completed",
			mangaType:   "manga",
			year:        2009,
			chapters:    43,
			genres:      []string{"Sci-Fi", "Thriller", "Mystery"},
		},
		{
			title:       "Jujutsu Kaisen",
			author:      "Gege Akutami",
			artist:      "Gege Akutami",
			description: "A high school student swallows a cursed finger and joins a school for jujutsu sorcerers.",
			status:      "ongoing",
			mangaType:   "manga",
			year:        2018,
			chapters:    270,
			genres:      []string{"Action", "Horror", "Supernatural"},
		},
		{
			title:       "Re:Zero",
			author:      "Tappei Nagatsuki",
			artist:      "Shinichirou Otsuka",
			description: "A boy is sent to a fantasy world and discovers he can return to the past when he dies.",
			status:      "ongoing",
			mangaType:   "manga",
			year:        2014,
			chapters:    150,
			genres:      []string{"Isekai", "Fantasy", "Drama"},
		},
		{
			title:       "Chainsaw Man",
			author:      "Tatsuki Fujimoto",
			artist:      "Tatsuki Fujimoto",
			description: "A poor boy becomes a devil hunter with a chainsaw devil living in his heart.",
			status:      "ongoing",
			mangaType:   "manga",
			year:        2018,
			chapters:    180,
			genres:      []string{"Action", "Horror", "Supernatural"},
		},
		{
			title:       "Fullmetal Alchemist",
			author:      "Hiromu Arakawa",
			artist:      "Hiromu Arakawa",
			description: "Two brothers seek the Philosopher's Stone to restore their bodies after a failed alchemical experiment.",
			status:      "completed",
			mangaType:   "manga",
			year:        2001,
			chapters:    116,
			genres:      []string{"Action", "Adventure", "Fantasy"},
		},
	}

	for _, m := range mangaList {
		mangaID := uuid.New().String()

		// Insert manga
		_, err := db.Exec(`
			INSERT INTO manga (id, title, author, artist, description, status, type, total_chapters, year, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			mangaID, m.title, m.author, m.artist, m.description,
			m.status, m.mangaType, m.chapters, m.year, time.Now(), time.Now(),
		)
		if err != nil {
			return err
		}

		// Get genre IDs and link to manga
		for _, genreName := range m.genres {
			var genreID string
			err := db.QueryRow("SELECT id FROM genres WHERE name = ?", genreName).Scan(&genreID)
			if err == nil {
				_, err = db.Exec(`
					INSERT INTO manga_genres (id, manga_id, genre_id, created_at)
					VALUES (?, ?, ?, ?)`,
					uuid.New().String(), mangaID, genreID, time.Now(),
				)
				if err != nil {
					return err
				}
			}
		}

		// Create external IDs entry
		_, err = db.Exec(`
			INSERT INTO manga_external_ids (manga_id, mangadex_id, primary_source, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)`,
			mangaID, uuid.New().String()[:8], "mangadex", time.Now(), time.Now(),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) seedReadingProgress() error {
	// Get test users
	rows, err := db.Query("SELECT id FROM users WHERE role = 'user' LIMIT 3")
	if err != nil {
		return err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		userIDs = append(userIDs, userID)
	}

	if len(userIDs) == 0 {
		return nil
	}

	// Get manga
	mangaRows, err := db.Query("SELECT id FROM manga LIMIT 5")
	if err != nil {
		return err
	}
	defer mangaRows.Close()

	var mangaIDs []string
	for mangaRows.Next() {
		var mangaID string
		if err := mangaRows.Scan(&mangaID); err != nil {
			return err
		}
		mangaIDs = append(mangaIDs, mangaID)
	}

	if len(mangaIDs) == 0 {
		return nil
	}

	// Create reading progress entries
	statuses := []string{"plan_to_read", "reading", "completed"}
	for i, userID := range userIDs {
		for j, mangaID := range mangaIDs {
			status := statuses[(i+j)%len(statuses)]
			currentChapter := (i + j + 1) * 10

			if status == "completed" {
				currentChapter = 100
			} else if status == "plan_to_read" {
				currentChapter = 0
			}

			_, err := db.Exec(`
				INSERT INTO reading_progress (id, user_id, manga_id, current_chapter, status, is_favorite, last_read_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				uuid.New().String(), userID, mangaID, currentChapter, status, j%2 == 0, time.Now(), time.Now(), time.Now(),
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
