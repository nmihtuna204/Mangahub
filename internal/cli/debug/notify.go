package debug

import (
	"encoding/json"
	"fmt"
	"net"

	"mangahub/internal/udp"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var notifyCmd = &cobra.Command{
	Use:   "notify",
	Short: "Send a test notification",
	Long:  "Send a broadcast notification via UDP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID, _ := cmd.Flags().GetString("manga-id")
		message, _ := cmd.Flags().GetString("message")
		notifType, _ := cmd.Flags().GetString("type")

		if mangaID == "" {
			return fmt.Errorf("--manga-id is required")
		}

		// Create notification
		notification := udp.Notification{
			Type:      notifType,
			MangaID:   mangaID,
			Message:   message,
			Timestamp: 0, // Server will likely set it or we set it here? Protocol says int64.
		}

		// Connect to UDP server
		host := viper.GetString("server.host")
		port := viper.GetInt("server.udp_port")
		serverAddr := fmt.Sprintf("%s:%d", host, port)

		addr, err := net.ResolveUDPAddr("udp", serverAddr)
		if err != nil {
			return fmt.Errorf("resolve addr: %w", err)
		}

		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			return fmt.Errorf("dial udp: %w", err)
		}
		defer conn.Close()

		// Prepare broadcast message
		jsonBytes, _ := json.Marshal(notification)
		msg := "BROADCAST " + string(jsonBytes)

		_, err = conn.Write([]byte(msg))
		if err != nil {
			return fmt.Errorf("send failed: %w", err)
		}

		fmt.Printf("✓ Notification sent to UDP server at %s\n", serverAddr)
		fmt.Printf("  Type: %s\n", notifType)
		fmt.Printf("  Manga: %s\n", mangaID)
		fmt.Printf("  Message: %s\n", message)

		return nil
	},
}

func init() {
	notifyCmd.Flags().String("manga-id", "", "Manga ID")
	notifyCmd.Flags().String("message", "New chapter released!", "Notification message")
	notifyCmd.Flags().String("type", "chapter_release", "Notification type")
	notifyCmd.MarkFlagRequired("manga-id")
	DebugCmd.AddCommand(notifyCmd)
}
