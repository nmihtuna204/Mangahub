package grpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mangahub/internal/grpc/pb"
	"mangahub/pkg/logger"
)

type Client struct {
	conn   *grpc.ClientConn
	client pb.MangaServiceClient
}

func NewClient(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	client := pb.NewMangaServiceClient(conn)
	return &Client{
		conn:   conn,
		client: client,
	}, nil
}

func (c *Client) GetManga(mangaID string) (*pb.MangaResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetManga(ctx, &pb.GetMangaRequest{
		MangaId: mangaID,
	})
	if err != nil {
		logger.Errorf("GetManga failed: %v", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) SearchManga(query string, limit int32, offset int32) (*pb.SearchResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.SearchManga(ctx, &pb.SearchRequest{
		Query:  query,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		logger.Errorf("SearchManga failed: %v", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) UpdateProgress(userID, mangaID string, chapter int32, status string) (*pb.ProgressResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.UpdateProgress(ctx, &pb.ProgressRequest{
		UserId:         userID,
		MangaId:        mangaID,
		CurrentChapter: chapter,
		Status:         status,
		Rating:         0,
	})
	if err != nil {
		logger.Errorf("UpdateProgress failed: %v", err)
		return nil, err
	}

	return resp, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
