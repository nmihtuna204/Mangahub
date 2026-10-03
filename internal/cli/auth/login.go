package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mangahub/internal/cli/apiutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to MangaHub",
	Long:  "Authenticate with your username and password to access MangaHub services",
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")

		if username == "" {
			fmt.Print("Username: ")
			fmt.Scanln(&username)
		}

		password, err := readPassword("Password: ")
		if err != nil {
			return err
		}

		body := map[string]string{
			"username": username,
			"password": password,
		}

		jsonBody, _ := json.Marshal(body)
		serverURL := fmt.Sprintf("http://%s:%d/auth/login",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"))

		resp, err := http.Post(serverURL, "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] == true {
			data := result["data"].(map[string]interface{})
			token := data["token"].(string)
			user := data["user"].(map[string]interface{})

			// Save token to config
			home, _ := os.UserHomeDir()
			configDir := filepath.Join(home, ".mangahub")
			os.MkdirAll(configDir, 0755)

			viper.Set("user.username", username)
			viper.Set("user.id", user["id"])
			viper.Set("user.token", token)
			viper.WriteConfigAs(filepath.Join(configDir, "config.yaml"))

			fmt.Println("✓ Login successful!")
			fmt.Printf("  Welcome back, %s!\n", username)
			fmt.Printf("  Token saved to: %s\n", filepath.Join(configDir, "config.yaml"))
		} else {
			return fmt.Errorf("login failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		return nil
	},
}

func init() {
	loginCmd.Flags().String("username", "", "Username")
	AuthCmd.AddCommand(loginCmd)
}
