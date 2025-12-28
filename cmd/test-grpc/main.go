// Package main - gRPC Protocol Manual Test
// Gọi gRPC methods để test inter-service communication
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mangahub/internal/grpc/pb"
)

func main() {
	host := flag.String("host", "localhost", "gRPC server host")
	port := flag.Int("port", 9092, "gRPC server port")
	method := flag.String("method", "get-manga", "Method to call: get-manga, search-manga, update-progress")
	mangaID := flag.String("manga", "5463cf5e-ec80-48ba-a3e2-04a8d825e555", "Manga ID (One Piece)")
	query := flag.String("query", "kimetsu", "Search query")
	userID := flag.String("user", "test-user", "User ID (for update-progress)")
	chapter := flag.Int("chapter", 100, "Chapter number (for update-progress)")
	statusFlag := flag.String("status", "reading", "Status (for update-progress)")
	flag.Parse()

	addr := fmt.Sprintf("%s:%d", *host, *port)
	fmt.Printf("🔗 Connecting to gRPC server at %s...\n", addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("✅ Connected!")

	client := pb.NewMangaServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch *method {
	case "get-manga":
		getMangas(ctx, client, *mangaID)
	case "search-manga":
		searchMangas(ctx, client, *query)
	case "update-progress":
		updateProgress(ctx, client, *userID, *mangaID, *chapter, *statusFlag)
	default:
		fmt.Printf("❌ Unknown method: %s\n", *method)
		fmt.Println("Available methods: get-manga, search-manga, update-progress")
	}
}

func getMangas(ctx context.Context, client pb.MangaServiceClient, mangaID string) {
	fmt.Printf("\n📤 Calling GetManga(id=%s)...\n", mangaID)

	resp, err := client.GetManga(ctx, &pb.GetMangaRequest{
		MangaId: mangaID,
	})
	if err != nil {
		fmt.Printf("❌ RPC failed: %v\n", err)
		return
	}

	fmt.Println("✅ Response received:\n")
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
}

func searchMangas(ctx context.Context, client pb.MangaServiceClient, query string) {
	fmt.Printf("\n📤 Calling SearchManga(query=%s, limit=10)...\n", query)

	resp, err := client.SearchManga(ctx, &pb.SearchRequest{
		Query:  query,
		Limit:  10,
		Offset: 0,
	})
	if err != nil {
		fmt.Printf("❌ RPC failed: %v\n", err)
		return
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
}

func updateProgress(ctx context.Context, client pb.MangaServiceClient, userID, mangaID string, chapter int, status string) {
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
		return
	}

	fmt.Println("✅ Progress updated!\n")
	fmt.Printf("   ID: %s\n", resp.Id)
	fmt.Printf("   User: %s\n", resp.UserId)
	fmt.Printf("   Manga: %s\n", resp.MangaId)
	fmt.Printf("   Chapter: %d\n", resp.CurrentChapter)
	fmt.Printf("   Status: %s\n", resp.Status)
	fmt.Printf("   Last Updated: %v\n", time.Unix(resp.Timestamp, 0))
}
