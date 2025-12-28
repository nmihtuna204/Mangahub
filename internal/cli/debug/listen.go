package debug

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mangahub/internal/tcp"
	"mangahub/internal/udp"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var listenCmd = &cobra.Command{
	Use:   "listen",
	Short: "Listen for TCP/UDP messages",
	Long:  "Connects to TCP and UDP servers and prints received messages to console",
	Run: func(cmd *cobra.Command, args []string) {
		tcpHost := viper.GetString("server.host")
		tcpPort := viper.GetInt("server.tcp_port")
		udpHost := viper.GetString("server.host")
		udpPort := viper.GetInt("server.udp_port")

		fmt.Printf("Starting listeners...\n")
		fmt.Printf("TCP Server: %s:%d\n", tcpHost, tcpPort)
		fmt.Printf("UDP Server: %s:%d\n", udpHost, udpPort)
		fmt.Println("Press Ctrl+C to exit")
		fmt.Println("----------------------------------------")

		// Start TCP Client
		go func() {
			client := tcp.NewClient(tcpHost, tcpPort)
			for {
				if err := client.Connect(); err != nil {
					fmt.Printf("[TCP] Connection failed: %v. Retrying in 5s...\n", err)
					time.Sleep(5 * time.Second)
					continue
				}
				fmt.Printf("[TCP] Connected!\n")

				decoder := json.NewDecoder(client.Conn)
				for {
					var update tcp.ProgressUpdate
					if err := decoder.Decode(&update); err != nil {
						fmt.Printf("[TCP] Disconnected: %v\n", err)
						client.Close()
						break
					}
					fmt.Printf("[TCP] Received: User=%s Manga=%s Chapter=%d\n",
						update.UserID, update.MangaID, update.Chapter)
				}
				time.Sleep(1 * time.Second)
			}
		}()

		// Start UDP Client
		go func() {
			client := udp.NewClient(udpHost, udpPort)
			client.OnNotification = func(n udp.Notification) {
				fmt.Printf("[UDP] Notification: [%s] %s (Manga: %s)\n",
					n.Type, n.Message, n.MangaID)
			}

			for {
				if err := client.Connect(); err != nil {
					fmt.Printf("[UDP] Connection failed: %v. Retrying in 5s...\n", err)
					time.Sleep(5 * time.Second)
					continue
				}
				// Keep running until error (which is handled inside client.listen usually, but client.listen is blocking?
				// Wait, udp.Client.listen is run in a goroutine inside Connect?
				// Let's check udp/client.go. Yes, go c.listen().
				// So Connect returns immediately.

				// We need to keep the main loop alive or monitor connection?
				// UDP is connectionless, so "Connect" just sets up the socket and registers.
				// If server restarts, we might need to re-register?
				// For now, just sleep.
				select {}
			}
		}()

		// Wait for interrupt
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nStopping listeners...")
	},
}

func init() {
	DebugCmd.AddCommand(listenCmd)
}
