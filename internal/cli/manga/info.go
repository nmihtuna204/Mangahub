package manga

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"mangahub/internal/cli/apiutil"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var infoCmd = &cobra.Command{
	Use:   "info <manga-id>",
	Short: "Show details for a manga",
	Long:  "Show full details for a manga: author, status, chapters, genres, rating and description",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		serverURL := fmt.Sprintf("http://%s:%d/manga/%s",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"),
			url.PathEscape(args[0]))

		resp, err := http.Get(serverURL)
		if err != nil {
			return fmt.Errorf("failed to get manga: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] != true {
			return fmt.Errorf("failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		m, _ := result["data"].(map[string]interface{})
		fmt.Fprintf(out, "\n%v\n", m["title"])
		fmt.Fprintf(out, "%s\n\n", strings.Repeat("=", len(fmt.Sprint(m["title"]))))
		fmt.Fprintf(out, "  Author:   %v\n", orDash(m["author"]))
		if artist := fmt.Sprint(m["artist"]); artist != "" && artist != fmt.Sprint(m["author"]) {
			fmt.Fprintf(out, "  Artist:   %s\n", artist)
		}
		fmt.Fprintf(out, "  Type:     %v\n", m["type"])
		fmt.Fprintf(out, "  Status:   %v\n", m["status"])
		if year, ok := m["year"].(float64); ok && year > 0 {
			fmt.Fprintf(out, "  Year:     %.0f\n", year)
		}
		if chapters, ok := m["total_chapters"].(float64); ok {
			fmt.Fprintf(out, "  Chapters: %.0f\n", chapters)
		}

		var genres []string
		if list, ok := m["genres"].([]interface{}); ok {
			for _, g := range list {
				if gm, ok := g.(map[string]interface{}); ok {
					genres = append(genres, fmt.Sprint(gm["name"]))
				}
			}
		}
		if len(genres) > 0 {
			fmt.Fprintf(out, "  Genres:   %s\n", strings.Join(genres, ", "))
		}

		count, _ := m["rating_count"].(float64)
		if count > 0 {
			avg, _ := m["average_rating"].(float64)
			fmt.Fprintf(out, "  Rating:   %.1f/10 (%.0f ratings)\n", avg, count)
		} else {
			fmt.Fprintf(out, "  Rating:   not rated yet\n")
		}

		if desc := fmt.Sprint(m["description"]); desc != "" {
			fmt.Fprintf(out, "\n  %s\n", desc)
		}
		fmt.Fprintf(out, "\n  ID: %v\n", m["id"])
		return nil
	},
}

func orDash(v interface{}) interface{} {
	if v == nil || v == "" {
		return "-"
	}
	return v
}

func init() {
	MangaCmd.AddCommand(infoCmd)
}
