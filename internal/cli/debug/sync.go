package debug

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"mangahub/internal/tcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Send a TCP sync message",
	Long:  "Send a progress update directly to the TCP sync server to test broadcasting",
	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID, _ := cmd.Flags().GetString("manga-id")
		chapter, _ := cmd.Flags().GetInt("chapter")
		userID, _ := cmd.Flags().GetString("user-id")

		if mangaID == "" {
			return fmt.Errorf("--manga-id is required")
		}

		// Create update message
		update := tcp.ProgressUpdate{
			UserID:    userID,
			MangaID:   mangaID,
			Chapter:   chapter,
			Timestamp: time.Now().Unix(),
		}

		// Connect to TCP server
		host := viper.GetString("server.host")
		port := viper.GetInt("server.tcp_port")
		serverAddr := fmt.Sprintf("%s:%d", host, port)

		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			return fmt.Errorf("failed to connect to TCP server at %s: %w", serverAddr, err)
		}
		defer conn.Close()

		// Send message
		data, err := json.Marshal(update)
		if err != nil {
			return fmt.Errorf("marshal failed: %w", err)
		}

		_, err = conn.Write(append(data, '\n'))
		if err != nil {
			return fmt.Errorf("send failed: %w", err)
		}

		fmt.Printf("✓ Sync message sent to TCP server at %s\n", serverAddr)
		fmt.Printf("  User: %s\n", userID)
		fmt.Printf("  Manga: %s\n", mangaID)
		fmt.Printf("  Chapter: %d\n", chapter)

		return nil
	},
}

func init() {
	syncCmd.Flags().String("manga-id", "", "Manga ID")
	syncCmd.Flags().Int("chapter", 1, "Chapter number")
	syncCmd.Flags().String("user-id", "cli-tester", "User ID")
	syncCmd.MarkFlagRequired("manga-id")
	DebugCmd.AddCommand(syncCmd)
}
