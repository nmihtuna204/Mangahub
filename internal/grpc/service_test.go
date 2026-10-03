package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "mangahub/internal/grpc/pb"
	"mangahub/internal/testutil"
	"mangahub/pkg/models"
)

func setup(t *testing.T) *MangaServiceServer {
	t.Helper()
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	testutil.AddManga(t, db.DB, "m2", "Bravo")
	testutil.AddGenre(t, db.DB, "m1", "Mecha", "mecha")
	return NewMangaServiceServer(db.DB)
}

func wantCode(t *testing.T, err error, code codes.Code) {
	t.Helper()
	if status.Code(err) != code {
		t.Errorf("got %v (code %v), want %v", err, status.Code(err), code)
	}
}

func TestGetManga(t *testing.T) {
	s := setup(t)
	ctx := context.Background()

	m, err := s.GetManga(ctx, &pb.GetMangaRequest{MangaId: "m1"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "Alpha" || len(m.Genres) != 1 || m.Genres[0].Slug != "mecha" {
		t.Errorf("got %+v, want Alpha with the mecha genre (slug included)", m)
	}

	_, err = s.GetManga(ctx, &pb.GetMangaRequest{MangaId: "missing"})
	wantCode(t, err, codes.NotFound)
	_, err = s.GetManga(ctx, &pb.GetMangaRequest{})
	wantCode(t, err, codes.InvalidArgument)
}

func TestSearchManga(t *testing.T) {
	s := setup(t)
	ctx := context.Background()

	resp, err := s.SearchManga(ctx, &pb.SearchRequest{Genres: []string{"mecha"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 || len(resp.Manga) != 1 || resp.Manga[0].Id != "m1" {
		t.Errorf("genre search = %+v, want only m1 (the genre filter used to be ignored)", resp)
	}

	resp, err = s.SearchManga(ctx, &pb.SearchRequest{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Limit != 100 || resp.Total != 2 {
		t.Errorf("limit clamp: got limit %d total %d, want 100 and 2", resp.Limit, resp.Total)
	}
}

// asU1 is a context for calls made as user u1 (what AuthInterceptor provides)
func asU1() context.Context {
	return ContextWithCaller(context.Background(), &models.UserProfile{ID: "u1", Username: "u1", Role: "user"})
}

func TestUpdateProgress_Validation(t *testing.T) {
	s := setup(t)
	ctx := asU1()

	cases := []struct {
		req  *pb.ProgressRequest
		code codes.Code
	}{
		{&pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: -5}, codes.InvalidArgument},
		{&pb.ProgressRequest{UserId: "u1", MangaId: "m1", Status: "bogus"}, codes.InvalidArgument},
		{&pb.ProgressRequest{MangaId: "m1"}, codes.InvalidArgument},
		{&pb.ProgressRequest{UserId: "nobody", MangaId: "m1"}, codes.NotFound},
		{&pb.ProgressRequest{UserId: "u1", MangaId: "missing"}, codes.NotFound},
	}
	for _, c := range cases {
		_, err := s.UpdateProgress(ctx, c.req)
		wantCode(t, err, c.code)
	}
}

func TestUpdateProgress_WritesByUsernameOrID(t *testing.T) {
	s := setup(t)
	ctx := asU1()

	// testutil users have username == id; look up by either
	resp, err := s.UpdateProgress(ctx, &pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: 3})
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentChapter != 3 || resp.Status != "reading" || resp.Timestamp == 0 {
		t.Errorf("got %+v, want chapter 3, default status reading, a timestamp", resp)
	}

	resp2, err := s.UpdateProgress(ctx, &pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: 4, Status: "on_hold"})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Id != resp.Id || resp2.CurrentChapter != 4 {
		t.Errorf("second write %+v should update the same row %s", resp2, resp.Id)
	}
}

// Audit calls (from the protocol bridge) must never write: the HTTP API has
// already saved the update, and a late re-write could roll a newer one back.
func TestUpdateProgress_AuditDoesNotWrite(t *testing.T) {
	s := setup(t)
	audit := metadata.NewIncomingContext(asU1(), metadata.Pairs(AuditMetadataKey, "true"))

	_, err := s.UpdateProgress(audit, &pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: 9})
	wantCode(t, err, codes.NotFound) // nothing stored yet, and the audit must not create it

	s.UpdateProgress(asU1(), &pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: 10})

	// A stale audit for chapter 9 arrives after chapter 10 was saved
	resp, err := s.UpdateProgress(audit, &pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: 9})
	if err != nil {
		t.Fatal(err)
	}
	if resp.CurrentChapter != 10 {
		t.Errorf("audit returned chapter %d, want the stored 10", resp.CurrentChapter)
	}
	var stored int
	s.db.QueryRow(`SELECT current_chapter FROM reading_progress WHERE user_id = 'u1' AND manga_id = 'm1'`).Scan(&stored)
	if stored != 10 {
		t.Errorf("stored chapter = %d after a stale audit, want 10", stored)
	}
}

// Users may only change their own progress; admins anyone's.
func TestUpdateProgress_Permissions(t *testing.T) {
	s := setup(t)
	testutil.AddUser(t, s.db, "u2")
	req := &pb.ProgressRequest{UserId: "u1", MangaId: "m1", CurrentChapter: 5}

	_, err := s.UpdateProgress(context.Background(), req)
	wantCode(t, err, codes.Unauthenticated) // no caller: refuse instead of trusting user_id

	other := ContextWithCaller(context.Background(), &models.UserProfile{ID: "u2", Role: "user"})
	_, err = s.UpdateProgress(other, req)
	wantCode(t, err, codes.PermissionDenied)

	admin := ContextWithCaller(context.Background(), &models.UserProfile{ID: "u2", Role: "admin"})
	if _, err := s.UpdateProgress(admin, req); err != nil {
		t.Errorf("admin updating another user's progress: %v", err)
	}
}
