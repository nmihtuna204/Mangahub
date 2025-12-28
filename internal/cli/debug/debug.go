package debug

import "github.com/spf13/cobra"

var DebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Debug and testing commands",
	Long:  "Tools for testing TCP/UDP protocols and debugging the system",
}
