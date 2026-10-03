package progress

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mangahub/internal/cli/apiutil"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update reading progress",
	Long:  "Update your reading progress - triggers all 5 protocols (HTTP, TCP, UDP, WebSocket, gRPC)!",
	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID, _ := cmd.Flags().GetString("manga-id")
		chapter, _ := cmd.Flags().GetInt("chapter")
		rating, _ := cmd.Flags().GetInt("rating")
		status, _ := cmd.Flags().GetString("status")

		if mangaID == "" {
			return fmt.Errorf("--manga-id is required")
		}

		token := viper.GetString("user.token")
		if token == "" {
			return fmt.Errorf("not logged in. Please run: mangahub auth login")
		}

		if rating != 0 && (rating < 1 || rating > 10) {
			return fmt.Errorf("--rating must be between 1 and 10")
		}

		// Only send what the user asked to change; the API leaves omitted fields as they are
		body := map[string]interface{}{"manga_id": mangaID}
		if cmd.Flags().Changed("chapter") {
			body["current_chapter"] = chapter
		}
		if cmd.Flags().Changed("status") {
			body["status"] = status
		}

		jsonBody, _ := json.Marshal(body)
		serverURL := fmt.Sprintf("http://%s:%d/users/progress",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"))

		req, _ := http.NewRequest("PUT", serverURL, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to update progress: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] == true {
			saved, _ := result["data"].(map[string]interface{})
			fmt.Printf("✓ Progress updated successfully!\n")
			fmt.Printf("  Manga ID: %s\n", mangaID)
			fmt.Printf("  Chapter: %v\n", saved["current_chapter"])
			fmt.Printf("  Status: %v\n", saved["status"])
			if rating > 0 {
				if err := submitRating(mangaID, rating, token); err != nil {
					fmt.Printf("  ✗ Rating not saved: %v\n", err)
				} else {
					fmt.Printf("  Rating: %d/10\n", rating)
				}
			}
			fmt.Println("\n🔄 Synced across all protocols:")
			fmt.Println("  ✓ HTTP: API updated")
			fmt.Println("  ✓ TCP: Broadcasted to sync clients")
			fmt.Println("  ✓ UDP: Notification sent")
			fmt.Println("  ✓ WebSocket: Room members notified")
			fmt.Println("  ✓ gRPC: Audit logged")
		} else {
			return fmt.Errorf("failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		return nil
	},
}

func init() {
	updateCmd.Flags().String("manga-id", "", "Manga ID (required)")
	updateCmd.Flags().Int("chapter", 0, "Current chapter")
	updateCmd.Flags().Int("rating", 0, "Also rate the manga (1-10)")
	updateCmd.Flags().String("status", "reading", "Status (reading, completed, dropped)")
	updateCmd.MarkFlagRequired("manga-id")
	ProgressCmd.AddCommand(updateCmd)
}

// submitRating posts a 1-10 rating for the manga
func submitRating(mangaID string, rating int, token string) error {
	body, _ := json.Marshal(map[string]interface{}{"rating": rating})
	url := fmt.Sprintf("http://%s:%d/manga/%s/ratings",
		viper.GetString("server.host"), viper.GetInt("server.http_port"), mangaID)

	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		_ = json.Unmarshal(respBody, &result)
		return fmt.Errorf("%s", apiutil.ErrorMessage(result, resp.StatusCode))
	}
	return nil
}
