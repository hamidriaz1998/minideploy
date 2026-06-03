package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the minideploy daemon",
	Long: `Starts the minideploy server-side daemon that manages deployments,
processes, and symlinks for all registered apps.

The daemon listens on 127.0.0.1:8443 by default.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		stateDir, _ := cmd.Flags().GetString("state-dir")

		if err := daemon.Run(port, stateDir); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().IntP("port", "p", 8443, "port to listen on")
	daemonCmd.Flags().StringP("state-dir", "d", "/var/lib/minideploy", "state directory")
}
