package progress

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"mangahub/internal/cli/apiutil"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View your reading progress",
	Long:  "Show reading progress for every manga in your library, or details for one with --manga-id",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		mangaID, _ := cmd.Flags().GetString("manga-id")

		token := viper.GetString("user.token")
		if token == "" {
			return fmt.Errorf("not logged in. Please run: mangahub auth login")
		}

		serverURL := fmt.Sprintf("http://%s:%d/users/library",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"))

		req, _ := http.NewRequest("GET", serverURL, nil)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to get progress: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] != true {
			return fmt.Errorf("failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		entries, _ := result["data"].([]interface{})
		if mangaID != "" {
			for _, e := range entries {
				entry, _ := e.(map[string]interface{})
				if entry["manga_id"] == mangaID {
					printDetail(out, entry)
					return nil
				}
			}
			return fmt.Errorf("manga %s is not in your library (add it with: mangahub library add --manga-id %s)", mangaID, mangaID)
		}

		if len(entries) == 0 {
			fmt.Fprintln(out, "Your library is empty. Add manga with: mangahub library add --manga-id <id>")
			return nil
		}
		fmt.Fprintf(out, "\nReading progress (%d manga):\n\n", len(entries))
		for _, e := range entries {
			entry, _ := e.(map[string]interface{})
			manga, _ := entry["manga"].(map[string]interface{})
			current, _ := entry["current_chapter"].(float64)
			total, _ := manga["total_chapters"].(float64)
			fav := "  "
			if entry["is_favorite"] == true {
				fav = "★ "
			}
			fmt.Fprintf(out, "%s%-40s %s  %-12v\n", fav, truncate(fmt.Sprint(manga["title"]), 40),
				progressBar(current, total), entry["status"])
		}
		return nil
	},
}

func printDetail(out io.Writer, entry map[string]interface{}) {
	manga, _ := entry["manga"].(map[string]interface{})
	current, _ := entry["current_chapter"].(float64)
	total, _ := manga["total_chapters"].(float64)

	fmt.Fprintf(out, "\n%v\n\n", manga["title"])
	fmt.Fprintf(out, "  Progress:  %s\n", progressBar(current, total))
	fmt.Fprintf(out, "  Status:    %v\n", entry["status"])
	fmt.Fprintf(out, "  Favorite:  %v\n", entry["is_favorite"] == true)
	for _, f := range []struct{ label, key string }{
		{"Started", "started_at"}, {"Completed", "completed_at"}, {"Last read", "last_read_at"},
	} {
		if v, ok := entry[f.key].(string); ok && len(v) >= 10 {
			fmt.Fprintf(out, "  %-10s %s\n", f.label+":", v[:10])
		}
	}
	fmt.Fprintf(out, "\n  Manga ID:  %v\n", entry["manga_id"])
}

// progressBar renders "[██████░░░░] ch 12/20 (60%)"; without a known total it shows just the chapter.
func progressBar(current, total float64) string {
	if total <= 0 {
		return fmt.Sprintf("ch %.0f", current)
	}
	pct := current / total
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * 10)
	return fmt.Sprintf("[%s%s] ch %.0f/%.0f (%.0f%%)",
		strings.Repeat("█", filled), strings.Repeat("░", 10-filled), current, total, pct*100)
}

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

func init() {
	viewCmd.Flags().String("manga-id", "", "Show details for one manga")
	ProgressCmd.AddCommand(viewCmd)
}
