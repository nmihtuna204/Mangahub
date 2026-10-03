// Package main - gRPC Protocol Manual Test
// Gọi gRPC methods để test inter-service communication
//
// Exits 1 when the call fails, so scripts can use it. Without -manga it looks
// up a real manga first (IDs are generated when the database is seeded), and
// without -user, update-progress updates the token's own user.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	pb "mangahub/internal/grpc/pb"
)

func main() {
	host := flag.String("host", "localhost", "gRPC server host")
	port := flag.Int("port", 9092, "gRPC server port")
	method := flag.String("method", "get-manga", "Method to call: get-manga, search-manga, update-progress")
	mangaID := flag.String("manga", "", "Manga ID (default: the first manga a search returns)")
	query := flag.String("query", "kimetsu", "Search query")
	userID := flag.String("user", "", "User ID or username for update-progress (default: the token's user)")
	chapter := flag.Int("chapter", 100, "Chapter number (for update-progress)")
	statusFlag := flag.String("status", "reading", "Status (for update-progress)")
	token := flag.String("token", "", "JWT from POST /auth/login (required for update-progress)")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("🔗 Connecting to gRPC server at %s...\n", addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fail("Connection failed: %v", err)
	}
	defer conn.Close()

	fmt.Println("✅ Connected!")

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ok bool
	switch *method {
	case "get-manga":
		ok = getMangas(ctx, client, pickManga(ctx, client, *mangaID))
	case "search-manga":
		ok = searchMangas(ctx, client, *query)
	case "update-progress":
		if *token == "" {
			fail("update-progress needs -token (log in via POST /auth/login and pass the token)")
		}
		user := *userID
		if user == "" {
			if user = tokenUserID(*token); user == "" {
				fail("could not read user_id from the token; pass -user")
			}
		}
		manga := pickManga(ctx, client, *mangaID)
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+*token)
		ok = updateProgress(ctx, client, user, manga, *chapter, *statusFlag)
	default:
		fmt.Println("Available methods: get-manga, search-manga, update-progress")
		fail("Unknown method: %s", *method)
	}
	if !ok {
		os.Exit(1)
	}
}

func fail(format string, args ...interface{}) {
	fmt.Printf("❌ "+format+"\n", args...)
	os.Exit(1)
}

// pickManga returns id, or the first manga in title order when id is empty.
func pickManga(ctx context.Context, client pb.MangaServiceClient, id string) string {
	if id != "" {
		return id
	}
	resp, err := client.SearchManga(ctx, &pb.SearchRequest{Limit: 1})
	if err != nil || len(resp.Manga) == 0 {
		fail("could not look up a manga ID (%v); pass -manga", err)
	}
	fmt.Printf("ℹ️  No -manga given, using %s (%s)\n", resp.Manga[0].Title, resp.Manga[0].Id)
	return resp.Manga[0].Id
}

// tokenUserID reads the user_id claim from a JWT without verifying it (the
// server does that); it only picks a default for -user.
func tokenUserID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		UserID string `json:"user_id"`
	}
	if json.Unmarshal(raw, &claims) != nil {
		return ""
	}
	return claims.UserID
}

func getMangas(ctx context.Context, client pb.MangaServiceClient, mangaID string) bool {
	fmt.Printf("\n📤 Calling GetManga(id=%s)...\n", mangaID)

	resp, err := client.GetManga(ctx, &pb.GetMangaRequest{
		MangaId: mangaID,
	})
	if err != nil {
		fmt.Printf("❌ RPC failed: %v\n", err)
		return false
	}

	fmt.Println("✅ Response received:")
	fmt.Printf("   ID: %s\n", resp.Id)
	fmt.Printf("   Title: %s\n", resp.Title)
	fmt.Printf("   Author: %s\n", resp.Author)
	fmt.Printf("   Status: %s\n", resp.Status)
	fmt.Printf("   Type: %s\n", resp.Type)
	fmt.Printf("   Chapters: %d\n", resp.TotalChapters)
	fmt.Printf("   Rating: %.2f (%d votes)\n", resp.AverageRating, resp.RatingCount)
	fmt.Printf("   Year: %d\n", resp.Year)

	if len(resp.Genres) > 0 {
		fmt.Println("   Genres:")
		for _, g := range resp.Genres {
			fmt.Printf("     - %s\n", g.Name)
		}
	}
	return true
}

func searchMangas(ctx context.Context, client pb.MangaServiceClient, query string) bool {
	fmt.Printf("\n📤 Calling SearchManga(query=%s, limit=10)...\n", query)

	resp, err := client.SearchManga(ctx, &pb.SearchRequest{
		Query:  query,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		fmt.Printf("❌ RPC failed: %v\n", err)
		return false
	}

	fmt.Printf("\n✅ Found %d results:\n\n", resp.Total)

	for i, manga := range resp.Manga {
		fmt.Printf("%d. %s\n", i+1, manga.Title)
		fmt.Printf("   ID: %s\n", manga.Id)
		fmt.Printf("   Author: %s\n", manga.Author)
		fmt.Printf("   Status: %s\n", manga.Status)
		fmt.Printf("   Chapters: %d\n", manga.TotalChapters)
		fmt.Println()
	}
	return true
}

func updateProgress(ctx context.Context, client pb.MangaServiceClient, userID, mangaID string, chapter int, status string) bool {
	fmt.Printf("\n📤 Calling UpdateProgress(user=%s, manga=%s, chapter=%d, status=%s)...\n",
		userID, mangaID, chapter, status)

	resp, err := client.UpdateProgress(ctx, &pb.ProgressRequest{
		UserId:         userID,
		MangaId:        mangaID,
		CurrentChapter: int32(chapter),
		Status:         status,
	})
	if err != nil {
		fmt.Printf("❌ RPC failed: %v\n", err)
		return false
	}

	fmt.Println("✅ Progress updated!")
	fmt.Printf("   ID: %s\n", resp.Id)
	fmt.Printf("   User: %s\n", resp.UserId)
	fmt.Printf("   Manga: %s\n", resp.MangaId)
	fmt.Printf("   Chapter: %d\n", resp.CurrentChapter)
	fmt.Printf("   Status: %s\n", resp.Status)
	fmt.Printf("   Last Updated: %v\n", time.Unix(resp.Timestamp, 0))
	return true
}
