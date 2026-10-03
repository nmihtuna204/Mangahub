package chat

import (
	"context"
	"testing"

	"mangahub/internal/testutil"
)

func TestEnsureRoomAndHistory(t *testing.T) {
	db := testutil.NewDB(t)
	testutil.AddUser(t, db.DB, "u1")
	testutil.AddManga(t, db.DB, "m1", "Alpha")
	repo := NewRepository(db.DB)
	ctx := context.Background()

	for _, id := range []string{"general", "manga_m1", "manga_missing", "general"} {
		if err := repo.EnsureRoom(ctx, id, "u1"); err != nil {
			t.Fatalf("EnsureRoom(%q): %v", id, err)
		}
	}

	room, err := repo.GetRoom(ctx, "manga_m1")
	if err != nil || room == nil {
		t.Fatalf("GetRoom(manga_m1) = %v, %v", room, err)
	}
	if room.RoomType != "manga" || room.MangaID == nil || *room.MangaID != "m1" || room.Name != "Alpha Discussion" {
		t.Errorf("manga room = %+v, want a manga room for m1 named 'Alpha Discussion'", room)
	}
	// A manga_ prefix without a real manga is just a general room
	if room, _ := repo.GetRoom(ctx, "manga_missing"); room == nil || room.RoomType != "general" || room.MangaID != nil {
		t.Errorf("manga_missing = %+v, want a general room", room)
	}

	// Messages need an existing room (FK); history comes back oldest first
	for _, text := range []string{"first", "second"} {
		if err := repo.SaveMessage(ctx, &Message{RoomID: "general", UserID: "u1", Content: text}); err != nil {
			t.Fatalf("SaveMessage: %v", err)
		}
	}
	msgs, total, err := repo.GetMessagesByRoom(ctx, "general", 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(msgs) != 2 || msgs[0].Content != "first" || msgs[0].Username != "u1" {
		t.Errorf("history = %+v (total %d)", msgs, total)
	}
}
