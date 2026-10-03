package manga

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for manga",
	Long:  "Search the manga catalog by title or author",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]

		limit, _ := cmd.Flags().GetInt("limit")
		status, _ := cmd.Flags().GetString("status")
		genre, _ := cmd.Flags().GetString("genre")

		// Build query
		params := url.Values{}
		params.Set("q", query)
		params.Set("limit", fmt.Sprintf("%d", limit))
		if status != "" {
			params.Set("status", status)
		}
		if genre != "" {
			params.Set("genre", genre)
		}

		serverURL := fmt.Sprintf("http://%s:%d/manga?%s",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"),
			params.Encode())

		resp, err := http.Get(serverURL)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] != true {
			return fmt.Errorf("search failed")
		}

		data, _ := result["data"].(map[string]interface{})
		manga, _ := data["data"].([]interface{})
		total, _ := data["total"].(float64)

		fmt.Printf("\nFound %d results:\n\n", int(total))

		for i, m := range manga {
			item, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			fmt.Printf("%d. %v\n", i+1, item["title"])
			fmt.Printf("   Author: %v\n", item["author"])
			fmt.Printf("   Status: %v\n", item["status"])
			if chapters, ok := item["total_chapters"].(float64); ok {
				fmt.Printf("   Chapters: %.0f\n", chapters)
			}
			fmt.Printf("   ID: %v\n\n", item["id"])
		}

		return nil
	},
}

func init() {
	searchCmd.Flags().Int("limit", 10, "Number of results")
	searchCmd.Flags().String("status", "", "Filter by status (ongoing, completed, hiatus)")
	searchCmd.Flags().String("genre", "", "Filter by genre name or slug (e.g. action, slice-of-life)")
	MangaCmd.AddCommand(searchCmd)
}
