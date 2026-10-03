package auth

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

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a new account",
	Long:  "Create a new MangaHub account with username, email, and password",
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")
		email, _ := cmd.Flags().GetString("email")

		if username == "" {
			fmt.Print("Username: ")
			fmt.Scanln(&username)
		}
		if email == "" {
			fmt.Print("Email: ")
			fmt.Scanln(&email)
		}

		password, err := readPassword("Password: ")
		if err != nil {
			return err
		}

		confirm, err := readPassword("Confirm password: ")
		if err != nil {
			return err
		}

		if password != confirm {
			return fmt.Errorf("passwords do not match")
		}

		body := map[string]string{
			"username": username,
			"email":    email,
			"password": password,
		}

		jsonBody, _ := json.Marshal(body)
		serverURL := fmt.Sprintf("http://%s:%d/auth/register",
			viper.GetString("server.host"),
			viper.GetInt("server.http_port"))

		resp, err := http.Post(serverURL, "application/json", bytes.NewReader(jsonBody))
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		if result["success"] == true {
			fmt.Println("✓ Account created successfully!")
			fmt.Printf("  Username: %s\n", username)
			fmt.Printf("  Email: %s\n", email)
			fmt.Println("\nNext: mangahub auth login --username " + username)
		} else {
			return fmt.Errorf("registration failed: %s", apiutil.ErrorMessage(result, resp.StatusCode))
		}

		return nil
	},
}

func init() {
	registerCmd.Flags().String("username", "", "Username")
	registerCmd.Flags().String("email", "", "Email address")
	AuthCmd.AddCommand(registerCmd)
}
