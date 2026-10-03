package library

import (
	"encoding/json"
	"fmt"
	"io"
	"mangahub/internal/cli/apiutil"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List your manga library",
	Long:  "View all manga in your library with reading progress",
	RunE: func(cmd *cobra.Command, args []string) error {
		token := viper.GetString("user.token")
		if token == "" {
			return fmt.Errorf("not logged in. Please run: mangahub auth login")
		}

		serverURL := fmt.Sprintf("http://%s:%d/users/library",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"))

		req, _ := http.NewRequest("GET", serverURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get library: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] == true {
			library := result["data"].([]interface{})

			fmt.Printf("\nYour Library (%d manga):\n\n", len(library))

			for i, item := range library {
				entry, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				// Progress fields live at the top level of each entry;
				// manga details are nested under "manga".
				title, author := "(unknown)", ""
				if manga, ok := entry["manga"].(map[string]interface{}); ok {
					if t, ok := manga["title"].(string); ok {
						title = t
					}
					if a, ok := manga["author"].(string); ok {
						author = a
					}
				}

				fmt.Printf("%d. %s\n", i+1, title)
				if author != "" {
					fmt.Printf("   Author: %s\n", author)
				}
				if status, ok := entry["status"].(string); ok {
					fmt.Printf("   Status: %s\n", status)
				}
				if chapter, ok := entry["current_chapter"].(float64); ok {
					fmt.Printf("   Progress: Chapter %.0f\n", chapter)
				}
				if fav, ok := entry["is_favorite"].(bool); ok && fav {
					fmt.Println("   ★ Favorite")
				}
				fmt.Println()
			}
		} else {
			return fmt.Errorf("failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		return nil
	},
}

func init() {
	LibraryCmd.AddCommand(listCmd)
}
