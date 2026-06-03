package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload artifacts to server via rsync",
	Long:  `Rsync the build artifacts to the server upload directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if configPath == "" {
			var err error
			configPath, err = client.FindConfig()
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				os.Exit(1)
			}
		}

		cfg, err := client.LoadConfig(configPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		if err := client.RunRsync(client.RsyncConfig{
			SSHUser:   cfg.Server.SSHUser,
			Host:      cfg.Server.Host,
			DeployDir: cfg.DeployPath,
			Artifacts: cfg.Artifacts,
		}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		fmt.Println("[upload] artifacts uploaded successfully")
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
}
