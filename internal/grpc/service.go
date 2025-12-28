// Package grpc - gRPC Service Implementation
// Implement Protocol Buffers RPCs cho internal services
// Chức năng:
//   - GetManga RPC: Lấy thông tin manga theo ID
//   - SearchManga RPC: Tìm kiếm manga với filters
//   - UpdateProgress RPC: Cập nhật reading progress
//   - High-performance binary protocol
//   - Type-safe communication với protobuf
//   - Reflection support cho debugging
package grpc

import (
	"context"
	"database/sql"
	"fmt"

	pb "mangahub/internal/grpc/pb"
	"mangahub/pkg/logger"
	"mangahub/pkg/models"
)

type MangaServiceServer struct {
	pb.UnimplementedMangaServiceServer
	db *sql.DB
}

func NewMangaServiceServer(db *sql.DB) *MangaServiceServer {
	return &MangaServiceServer{
		db: db,
	}
}

// GetManga retrieves a single manga by ID
func (s *MangaServiceServer) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	// Protocol trace logging
	logger.GRPC("GetManga", "manga_id="+req.MangaId, 0)

	var manga models.Manga
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, author, artist, description, cover_url, status, type,
		       total_chapters, average_rating, rating_count, year
		FROM manga WHERE id = ?`, req.MangaId)

	if err := row.Scan(
		&manga.ID, &manga.Title, &manga.Author, &manga.Artist, &manga.Description,
		&manga.CoverURL, &manga.Status, &manga.Type,
		&manga.TotalChapters, &manga.AverageRating, &manga.RatingCount, &manga.Year,
	); err != nil {
		if err == sql.ErrNoRows {
			logger.Warnf("gRPC: Manga not found: %s", req.MangaId)
			return nil, fmt.Errorf("manga not found: %s", req.MangaId)
		}
		logger.Errorf("gRPC: Database error: %v", err)
		return nil, err
	}

	// Load genres via separate query (limit to 20 for safety)
	var genres []*pb.Genre
	genreRows, err := s.db.QueryContext(ctx, `
		SELECT g.id, g.name FROM genres g
		INNER JOIN manga_genres mg ON g.id = mg.genre_id
		WHERE mg.manga_id = ?
		LIMIT 20`, req.MangaId)
	if err == nil {
		defer genreRows.Close()
		for genreRows.Next() {
			var gid, gname string
			if err := genreRows.Scan(&gid, &gname); err == nil {
				genres = append(genres, &pb.Genre{
					Id:   gid,
					Name: gname,
				})
			}
		}
	}

	resp := &pb.MangaResponse{
		Id:            manga.ID,
		Title:         manga.Title,
		Author:        manga.Author,
		Artist:        manga.Artist,
		Description:   manga.Description,
		CoverUrl:      manga.CoverURL,
		Status:        manga.Status,
		Type:          manga.Type,
		TotalChapters: int32(manga.TotalChapters),
		AverageRating: manga.AverageRating,
		RatingCount:   int32(manga.RatingCount),
		Year:          int32(manga.Year),
		Genres:        genres,
	}

	return resp, nil
}

// SearchManga searches for manga with filters
func (s *MangaServiceServer) SearchManga(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	// Protocol trace logging
	logger.GRPC("SearchManga", fmt.Sprintf("query=%s limit=%d offset=%d", req.Query, req.Limit, req.Offset), 0)

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	// Build WHERE clause
	conditions := []string{"1=1"}
	args := []interface{}{}

	if req.Query != "" {
		conditions = append(conditions, "(title LIKE ? OR author LIKE ?)")
		q := "%" + req.Query + "%"
		args = append(args, q, q)
	}

	if req.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, req.Status)
	}

	where := ""
	for i, cond := range conditions {
		if i == 0 {
			where = cond
		} else {
			where += " AND " + cond
		}
	}

	// Get total count
	var total int32
	countSQL := "SELECT COUNT(*) FROM manga WHERE " + where
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		logger.Errorf("gRPC: Count query error: %v", err)
		return nil, err
	}

	// Get paginated results
	listSQL := fmt.Sprintf(`
		SELECT id, title, author, artist, description, cover_url, status, type,
		       total_chapters, average_rating, rating_count, year
		FROM manga
		WHERE %s
		ORDER BY title ASC
		LIMIT ? OFFSET ?`, where)

	argsWithPaging := append(args, req.Limit, req.Offset)

	rows, err := s.db.QueryContext(ctx, listSQL, argsWithPaging...)
	if err != nil {
		logger.Errorf("gRPC: Query error: %v", err)
		return nil, err
	}
	defer rows.Close()

	var mangaList []*pb.MangaResponse
	for rows.Next() {
		var manga models.Manga
		if err := rows.Scan(
			&manga.ID, &manga.Title, &manga.Author, &manga.Artist, &manga.Description,
			&manga.CoverURL, &manga.Status, &manga.Type,
			&manga.TotalChapters, &manga.AverageRating, &manga.RatingCount, &manga.Year,
		); err != nil {
			logger.Errorf("gRPC: Scan error: %v", err)
			return nil, err
		}

		// Load genres for each manga
		var genres []*pb.Genre
		genreRows, err := s.db.QueryContext(ctx, `
			SELECT g.id, g.name, g.slug FROM genres g
			INNER JOIN manga_genres mg ON g.id = mg.genre_id
			WHERE mg.manga_id = ?`, manga.ID)
		if err == nil {
			defer genreRows.Close()
			for genreRows.Next() {
				var genre pb.Genre
				if err := genreRows.Scan(&genre.Id, &genre.Name, &genre.Slug); err == nil {
					genres = append(genres, &genre)
				}
			}
		}

		mangaList = append(mangaList, &pb.MangaResponse{
			Id:            manga.ID,
			Title:         manga.Title,
			Author:        manga.Author,
			Artist:        manga.Artist,
			Description:   manga.Description,
			CoverUrl:      manga.CoverURL,
			Status:        manga.Status,
			Type:          manga.Type,
			TotalChapters: int32(manga.TotalChapters),
			AverageRating: manga.AverageRating,
			RatingCount:   int32(manga.RatingCount),
			Year:          int32(manga.Year),
			Genres:        genres,
		})
	}

	logger.Infof("gRPC: SearchManga returned %d results", len(mangaList))

	return &pb.SearchResponse{
		Manga:  mangaList,
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

// UpdateProgress updates user reading progress
func (s *MangaServiceServer) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	logger.Infof("gRPC: UpdateProgress called for user=%s, manga=%s, chapter=%d",
		req.UserId, req.MangaId, req.CurrentChapter)

	// Check if user_id is a username and convert to UUID
	userID := req.UserId
	var userUUID string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE id = ? OR username = ?", req.UserId, req.UserId).Scan(&userUUID)
	if err != nil {
		logger.Errorf("gRPC: User not found: %v", err)
		return nil, fmt.Errorf("user not found: %s", req.UserId)
	}
	userID = userUUID

	// Check if progress record exists
	var existingID string
	err = s.db.QueryRowContext(ctx,
		"SELECT id FROM reading_progress WHERE user_id = ? AND manga_id = ?",
		userID, req.MangaId,
	).Scan(&existingID)

	if err != nil && err != sql.ErrNoRows {
		logger.Errorf("gRPC: Query error: %v", err)
		return nil, err
	}

	if err == sql.ErrNoRows {
		// Insert new progress record
		newID := fmt.Sprintf("%s-%s", userID, req.MangaId)
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO reading_progress
			(id, user_id, manga_id, current_chapter, status, last_read_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, datetime('now'), datetime('now'), datetime('now'))`,
			newID, userID, req.MangaId, req.CurrentChapter, req.Status,
		)
		if err != nil {
			logger.Errorf("gRPC: Insert error: %v", err)
			return nil, err
		}
		existingID = newID
	} else {
		// Update existing progress
		_, err = s.db.ExecContext(ctx, `
			UPDATE reading_progress
			SET current_chapter = ?, status = ?, last_read_at = datetime('now'), 
			    updated_at = datetime('now')
			WHERE id = ?`,
			req.CurrentChapter, req.Status, existingID,
		)
		if err != nil {
			logger.Errorf("gRPC: Update error: %v", err)
			return nil, err
		}
	}

	logger.Infof("gRPC: UpdateProgress completed for progress_id=%s", existingID)

	return &pb.ProgressResponse{
		Id:             existingID,
		UserId:         userID,
		MangaId:        req.MangaId,
		CurrentChapter: req.CurrentChapter,
		Status:         req.Status,
		Timestamp:      0, // Set by server
	}, nil
}
