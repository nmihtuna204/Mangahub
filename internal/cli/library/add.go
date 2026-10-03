package library

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

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add manga to your library",
	Long:  "Add a manga to your library with optional status and current chapter",
	RunE: func(cmd *cobra.Command, args []string) error {
		mangaID, _ := cmd.Flags().GetString("manga-id")
		status, _ := cmd.Flags().GetString("status")
		chapter, _ := cmd.Flags().GetInt("chapter")

		if mangaID == "" {
			return fmt.Errorf("--manga-id is required")
		}

		token := viper.GetString("user.token")
		if token == "" {
			return fmt.Errorf("not logged in. Please run: mangahub auth login")
		}

		// If the manga is already in the library, only the flags given here change
		body := map[string]interface{}{
			"manga_id": mangaID,
			"status":   status,
		}
		if cmd.Flags().Changed("chapter") {
			body["current_chapter"] = chapter
		}

		jsonBody, _ := json.Marshal(body)
		serverURL := fmt.Sprintf("http://%s:%d/users/library",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"))

		req, _ := http.NewRequest("POST", serverURL, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to add manga: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] == true {
			fmt.Printf("✓ Manga added to library\n")
			fmt.Printf("  Manga ID: %s\n", mangaID)
			fmt.Printf("  Status: %s\n", status)
			fmt.Printf("  Current chapter: %d\n", chapter)
		} else {
			return fmt.Errorf("failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		return nil
	},
}

func init() {
	addCmd.Flags().String("manga-id", "", "Manga ID (required)")
	addCmd.Flags().String("status", "reading", "Status (reading, completed, plan_to_read)")
	addCmd.Flags().Int("chapter", 0, "Current chapter")
	addCmd.MarkFlagRequired("manga-id")
	LibraryCmd.AddCommand(addCmd)
}
