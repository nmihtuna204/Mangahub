package root

import (
	"fmt"
	"os"

	"mangahub/internal/cli/auth"
	"mangahub/internal/cli/config"
	"mangahub/internal/cli/debug"
	"mangahub/internal/cli/library"
	"mangahub/internal/cli/manga"
	"mangahub/internal/cli/progress"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "mangahub",
	Short: "MangaHub - Manga tracking and synchronization system",
	Long: `MangaHub is a manga tracking system with real-time synchronization
using HTTP, TCP, UDP, WebSocket, and gRPC protocols.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.mangahub/config.yaml)")
	rootCmd.PersistentFlags().Bool("verbose", false, "Enable verbose output")

	// Add command groups
	rootCmd.AddCommand(auth.AuthCmd)
	rootCmd.AddCommand(manga.MangaCmd)
	rootCmd.AddCommand(library.LibraryCmd)
	rootCmd.AddCommand(progress.ProgressCmd)
	rootCmd.AddCommand(config.ConfigCmd)
	rootCmd.AddCommand(debug.DebugCmd)

	// Version command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("MangaHub CLI v1.0.0")
			fmt.Println("Phase 8: CLI Tool Implementation")
		},
	})
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		viper.AddConfigPath(home + "/.mangahub")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.http_port", 8080)
	viper.SetDefault("server.tcp_port", 9090)
	viper.SetDefault("server.udp_port", 9091)
	viper.SetDefault("server.grpc_port", 9092)

	viper.ReadInConfig()
}
