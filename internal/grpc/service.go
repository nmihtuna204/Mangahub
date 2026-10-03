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
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "mangahub/internal/grpc/pb"
	"mangahub/internal/manga"
	"mangahub/pkg/logger"
	"mangahub/pkg/models"
)

var validStatuses = map[string]bool{
	"plan_to_read": true, "reading": true, "completed": true, "on_hold": true, "dropped": true,
}

type MangaServiceServer struct {
	pb.UnimplementedMangaServiceServer
	db    *sql.DB
	manga manga.Repository
}

func NewMangaServiceServer(db *sql.DB) *MangaServiceServer {
	return &MangaServiceServer{
		db:    db,
		manga: manga.NewRepository(db),
	}
}

// GetManga retrieves a single manga by ID
func (s *MangaServiceServer) GetManga(ctx context.Context, req *pb.GetMangaRequest) (*pb.MangaResponse, error) {
	// Protocol trace logging
	logger.GRPC("GetManga", "manga_id="+req.MangaId, 0)

	if req.MangaId == "" {
		return nil, status.Error(codes.InvalidArgument, "manga_id is required")
	}

	m, err := s.manga.GetByID(ctx, req.MangaId)
	if err != nil {
		var appErr *models.AppError
		if errors.As(err, &appErr) && appErr.StatusCode == 404 {
			logger.Warnf("gRPC: Manga not found: %s", req.MangaId)
			return nil, status.Errorf(codes.NotFound, "manga not found: %s", req.MangaId)
		}
		logger.Errorf("gRPC: Database error: %v", err)
		return nil, status.Error(codes.Internal, "failed to load manga")
	}
	return toPB(*m), nil
}

// SearchManga searches for manga with filters
func (s *MangaServiceServer) SearchManga(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	// Protocol trace logging
	logger.GRPC("SearchManga", fmt.Sprintf("query=%s limit=%d offset=%d", req.Query, req.Limit, req.Offset), 0)

	search := models.MangaSearchRequest{
		Query:  req.Query,
		Genres: req.Genres,
		Status: req.Status,
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}
	if err := models.ValidateMangaSearch(&search); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	list, total, err := s.manga.List(ctx, search)
	if err != nil {
		logger.Errorf("gRPC: Search error: %v", err)
		return nil, status.Error(codes.Internal, "search failed")
	}

	mangaList := make([]*pb.MangaResponse, 0, len(list))
	for _, m := range list {
		mangaList = append(mangaList, toPB(m))
	}

	logger.Infof("gRPC: SearchManga returned %d results", len(mangaList))

	return &pb.SearchResponse{
		Manga:  mangaList,
		Total:  int32(total),
		Limit:  int32(search.Limit),
		Offset: int32(search.Offset),
	}, nil
}

func toPB(m models.Manga) *pb.MangaResponse {
	genres := make([]*pb.Genre, 0, len(m.Genres))
	for _, g := range m.Genres {
		genres = append(genres, &pb.Genre{Id: g.ID, Name: g.Name, Slug: g.Slug})
	}
	return &pb.MangaResponse{
		Id:            m.ID,
		Title:         m.Title,
		Author:        m.Author,
		Artist:        m.Artist,
		Description:   m.Description,
		CoverUrl:      m.CoverURL,
		Status:        m.Status,
		Type:          m.Type,
		TotalChapters: int32(m.TotalChapters),
		AverageRating: m.AverageRating,
		RatingCount:   int32(m.RatingCount),
		Year:          int32(m.Year),
		Genres:        genres,
	}
}

// UpdateProgress updates user reading progress
func (s *MangaServiceServer) UpdateProgress(ctx context.Context, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	logger.Infof("gRPC: UpdateProgress called for user=%s, manga=%s, chapter=%d",
		req.UserId, req.MangaId, req.CurrentChapter)

	if req.UserId == "" || req.MangaId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and manga_id are required")
	}
	if req.CurrentChapter < 0 {
		return nil, status.Error(codes.InvalidArgument, "current_chapter must be >= 0")
	}
	if req.Status == "" {
		req.Status = "reading"
	}
	if !validStatuses[req.Status] {
		return nil, status.Errorf(codes.InvalidArgument, "invalid status %q", req.Status)
	}

	// Accept either the user's UUID or username
	var userID string
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE id = ? OR username = ?", req.UserId, req.UserId).Scan(&userID)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "user not found: %s", req.UserId)
	}
	if err != nil {
		logger.Errorf("gRPC: User lookup error: %v", err)
		return nil, status.Error(codes.Internal, "failed to look up user")
	}

	// Callers may only change their own progress (admins anyone's). The
	// caller comes from AuthInterceptor; without it, refuse rather than trust req.UserId.
	caller := CallerFromContext(ctx)
	if caller == nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	if caller.ID != userID && caller.Role != "admin" {
		return nil, status.Error(codes.PermissionDenied, "you can only update your own progress")
	}

	var one int
	err = s.db.QueryRowContext(ctx, "SELECT 1 FROM manga WHERE id = ?", req.MangaId).Scan(&one)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "manga not found: %s", req.MangaId)
	}
	if err != nil {
		logger.Errorf("gRPC: Manga lookup error: %v", err)
		return nil, status.Error(codes.Internal, "failed to look up manga")
	}

	if isAuditOnly(ctx) {
		return s.auditProgress(ctx, userID, req)
	}

	// Upsert keyed on the (user_id, manga_id) unique constraint.
	// Timestamps come from Go (not SQLite's datetime('now'), which is UTC text)
	// so they sort consistently with rows written by the HTTP API.
	now := time.Now()
	newID := fmt.Sprintf("%s-%s", userID, req.MangaId)
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO reading_progress
		(id, user_id, manga_id, current_chapter, status, last_read_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, manga_id) DO UPDATE SET
			current_chapter = excluded.current_chapter,
			status          = excluded.status,
			last_read_at    = excluded.last_read_at,
			updated_at      = excluded.updated_at`,
		newID, userID, req.MangaId, req.CurrentChapter, req.Status, now, now, now,
	)
	if err != nil {
		logger.Errorf("gRPC: Upsert error: %v", err)
		return nil, status.Error(codes.Internal, "failed to save progress")
	}

	var progressID string
	if err := s.db.QueryRowContext(ctx,
		"SELECT id FROM reading_progress WHERE user_id = ? AND manga_id = ?", userID, req.MangaId,
	).Scan(&progressID); err != nil {
		logger.Errorf("gRPC: Progress lookup error: %v", err)
		return nil, status.Error(codes.Internal, "failed to load progress")
	}

	logger.Infof("gRPC: UpdateProgress completed for progress_id=%s", progressID)

	return &pb.ProgressResponse{
		Id:             progressID,
		UserId:         userID,
		MangaId:        req.MangaId,
		CurrentChapter: req.CurrentChapter,
		Status:         req.Status,
		Timestamp:      time.Now().Unix(),
	}, nil
}

// AuditMetadataKey marks an UpdateProgress call as an audit of an update that
// the HTTP API has already saved. The server then records the audit entry and
// returns the stored progress without writing: re-writing would race with
// newer HTTP updates and briefly roll the chapter back.
const AuditMetadataKey = "x-mangahub-audit"

func isAuditOnly(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	vals := md.Get(AuditMetadataKey)
	return len(vals) > 0 && vals[0] == "true"
}

func (s *MangaServiceServer) auditProgress(ctx context.Context, userID string, req *pb.ProgressRequest) (*pb.ProgressResponse, error) {
	var progressID, storedStatus string
	var storedChapter int32
	err := s.db.QueryRowContext(ctx,
		"SELECT id, current_chapter, status FROM reading_progress WHERE user_id = ? AND manga_id = ?",
		userID, req.MangaId,
	).Scan(&progressID, &storedChapter, &storedStatus)
	if err == sql.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "no progress for user %s on manga %s", userID, req.MangaId)
	}
	if err != nil {
		logger.Errorf("gRPC: Audit lookup error: %v", err)
		return nil, status.Error(codes.Internal, "failed to load progress")
	}

	logger.Infof("gRPC: AUDIT progress user=%s manga=%s reported_chapter=%d stored_chapter=%d status=%s",
		userID, req.MangaId, req.CurrentChapter, storedChapter, storedStatus)

	return &pb.ProgressResponse{
		Id:             progressID,
		UserId:         userID,
		MangaId:        req.MangaId,
		CurrentChapter: storedChapter,
		Status:         storedStatus,
		Timestamp:      time.Now().Unix(),
	}, nil
}
