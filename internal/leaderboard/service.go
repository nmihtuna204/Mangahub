// Package leaderboard - Leaderboard Service
// Business logic layer cho leaderboard system
// Chức năng:
//   - Top rated manga
//   - Most active users
//   - Trending manga (most reads/ratings recently)
package leaderboard

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MangaLeaderboardEntry represents a manga in the leaderboard
type MangaLeaderboardEntry struct {
	Rank          int     `json:"rank"`
	MangaID       string  `json:"manga_id"`
	Title         string  `json:"title"`
	CoverURL      string  `json:"cover_url,omitempty"`
	Author        string  `json:"author,omitempty"`
	AverageRating float64 `json:"average_rating"`
	TotalRatings  int     `json:"total_ratings"`
	TotalReaders  int     `json:"total_readers"`
}

// UserLeaderboardEntry represents a user in the leaderboard
type UserLeaderboardEntry struct {
	Rank           int    `json:"rank"`
	UserID         string `json:"user_id"`
	Username       string `json:"username"`
	DisplayName    string `json:"display_name"`
	AvatarURL      string `json:"avatar_url,omitempty"`
	MangaCompleted int    `json:"manga_completed"`
	ChaptersRead   int    `json:"chapters_read"`
	TotalRatings   int    `json:"total_ratings"`
	TotalComments  int    `json:"total_comments"`
	Score          int    `json:"score"` // Computed engagement score
}

// LeaderboardResponse contains leaderboard data
type LeaderboardResponse struct {
	Type      string      `json:"type"` // manga, users, trending
	Period    string      `json:"period,omitempty"` // all_time, weekly, monthly
	Entries   interface{} `json:"entries"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Service defines business operations for leaderboards
type Service interface {
	// GetTopRatedManga returns manga sorted by rating
	GetTopRatedManga(ctx context.Context, limit, offset int) (*LeaderboardResponse, error)

	// GetMostActiveUsers returns users sorted by activity
	GetMostActiveUsers(ctx context.Context, limit, offset int) (*LeaderboardResponse, error)

	// GetTrendingManga returns manga with most activity recently
	GetTrendingManga(ctx context.Context, limit, offset int, days int) (*LeaderboardResponse, error)
}

type service struct {
	db *sql.DB
}

// NewService creates a new leaderboard service
func NewService(db *sql.DB) Service {
	return &service{db: db}
}

// GetTopRatedManga returns manga sorted by weighted rating
// Uses Bayesian average to balance popular and highly-rated manga
func (s *service) GetTopRatedManga(ctx context.Context, limit, offset int) (*LeaderboardResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	// Query manga with their rating stats and reader counts
	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			m.id, m.title, m.cover_url, m.author,
			COALESCE(AVG(r.overall_rating), 0) as avg_rating,
			COUNT(DISTINCT r.id) as total_ratings,
			COUNT(DISTINCT p.user_id) as total_readers
		FROM manga m
		LEFT JOIN manga_ratings r ON m.id = r.manga_id
		LEFT JOIN reading_progress p ON m.id = p.manga_id
		GROUP BY m.id
		HAVING COUNT(DISTINCT r.id) >= 1
		ORDER BY avg_rating DESC, total_ratings DESC
		LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get top rated manga: %w", err)
	}
	defer rows.Close()

	var entries []MangaLeaderboardEntry
	rank := offset + 1
	for rows.Next() {
		var e MangaLeaderboardEntry
		var coverURL, author sql.NullString

		err := rows.Scan(
			&e.MangaID, &e.Title, &coverURL, &author,
			&e.AverageRating, &e.TotalRatings, &e.TotalReaders,
		)
		if err != nil {
			return nil, fmt.Errorf("scan manga entry: %w", err)
		}

		e.Rank = rank
		e.CoverURL = coverURL.String
		e.Author = author.String
		entries = append(entries, e)
		rank++
	}

	return &LeaderboardResponse{
		Type:      "top_rated",
		Period:    "all_time",
		Entries:   entries,
		UpdatedAt: time.Now(),
	}, nil
}

// GetMostActiveUsers returns users sorted by engagement score
// Score = completed*10 + chapters*1 + ratings*5 + comments*3
func (s *service) GetMostActiveUsers(ctx context.Context, limit, offset int) (*LeaderboardResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			u.id, u.username, u.display_name, COALESCE(u.avatar_url, ''),
			COALESCE(completed.cnt, 0) as manga_completed,
			COALESCE(chapters.total, 0) as chapters_read,
			COALESCE(ratings.cnt, 0) as total_ratings,
			COALESCE(comments.cnt, 0) as total_comments
		FROM users u
		LEFT JOIN (
			SELECT user_id, COUNT(*) as cnt 
			FROM reading_progress WHERE status = 'completed' 
			GROUP BY user_id
		) completed ON u.id = completed.user_id
		LEFT JOIN (
			SELECT user_id, SUM(current_chapter) as total 
			FROM reading_progress 
			GROUP BY user_id
		) chapters ON u.id = chapters.user_id
		LEFT JOIN (
			SELECT user_id, COUNT(*) as cnt 
			FROM manga_ratings 
			GROUP BY user_id
		) ratings ON u.id = ratings.user_id
		LEFT JOIN (
			SELECT user_id, COUNT(*) as cnt 
			FROM comments WHERE is_deleted = 0 
			GROUP BY user_id
		) comments ON u.id = comments.user_id
		WHERE u.is_active = 1
		ORDER BY 
			(COALESCE(completed.cnt, 0) * 10 + 
			 COALESCE(chapters.total, 0) + 
			 COALESCE(ratings.cnt, 0) * 5 + 
			 COALESCE(comments.cnt, 0) * 3) DESC
		LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get most active users: %w", err)
	}
	defer rows.Close()

	var entries []UserLeaderboardEntry
	rank := offset + 1
	for rows.Next() {
		var e UserLeaderboardEntry

		err := rows.Scan(
			&e.UserID, &e.Username, &e.DisplayName, &e.AvatarURL,
			&e.MangaCompleted, &e.ChaptersRead, &e.TotalRatings, &e.TotalComments,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user entry: %w", err)
		}

		e.Rank = rank
		// Calculate engagement score
		e.Score = e.MangaCompleted*10 + e.ChaptersRead + e.TotalRatings*5 + e.TotalComments*3
		entries = append(entries, e)
		rank++
	}

	return &LeaderboardResponse{
		Type:      "most_active",
		Period:    "all_time",
		Entries:   entries,
		UpdatedAt: time.Now(),
	}, nil
}

// GetTrendingManga returns manga with most activity in last N days
// Activity = new ratings + new library adds + comments
func (s *service) GetTrendingManga(ctx context.Context, limit, offset int, days int) (*LeaderboardResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if days <= 0 {
		days = 7 // Default to weekly trending
	}

	// Calculate date threshold
	threshold := time.Now().AddDate(0, 0, -days)

	rows, err := s.db.QueryContext(ctx, `
		SELECT 
			m.id, m.title, m.cover_url, m.author,
			COALESCE(AVG(r.overall_rating), 0) as avg_rating,
			COUNT(DISTINCT r.id) as total_ratings,
			COUNT(DISTINCT p.user_id) as total_readers
		FROM manga m
		LEFT JOIN manga_ratings r ON m.id = r.manga_id AND r.created_at >= ?
		LEFT JOIN reading_progress p ON m.id = p.manga_id AND p.created_at >= ?
		GROUP BY m.id
		HAVING (COUNT(DISTINCT r.id) + COUNT(DISTINCT p.user_id)) >= 1
		ORDER BY (COUNT(DISTINCT r.id) + COUNT(DISTINCT p.user_id)) DESC
		LIMIT ? OFFSET ?`, threshold, threshold, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("get trending manga: %w", err)
	}
	defer rows.Close()

	var entries []MangaLeaderboardEntry
	rank := offset + 1
	for rows.Next() {
		var e MangaLeaderboardEntry
		var coverURL, author sql.NullString

		err := rows.Scan(
			&e.MangaID, &e.Title, &coverURL, &author,
			&e.AverageRating, &e.TotalRatings, &e.TotalReaders,
		)
		if err != nil {
			return nil, fmt.Errorf("scan trending entry: %w", err)
		}

		e.Rank = rank
		e.CoverURL = coverURL.String
		e.Author = author.String
		entries = append(entries, e)
		rank++
	}

	period := "weekly"
	if days == 30 {
		period = "monthly"
	}

	return &LeaderboardResponse{
		Type:      "trending",
		Period:    period,
		Entries:   entries,
		UpdatedAt: time.Now(),
	}, nil
}
