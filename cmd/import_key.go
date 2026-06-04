package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/daemon"
)

func init() {
	importKeyCmd.Flags().String("state-dir", daemon.DefaultStateDir, "state directory")
	daemonCmd.AddCommand(importKeyCmd)
}

var importKeyCmd = &cobra.Command{
	Use:   "import-key <hash>",
	Short: "Import an API key hash into the daemon database",
	Long: `Low-level command to directly insert an API key hash into the
daemon's SQLite database. Used by init-server to pre-seed the
admin key before the daemon starts.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]
		stateDir, _ := cmd.Flags().GetString("state-dir")

		if err := daemon.SeedKey(stateDir, hash); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}
