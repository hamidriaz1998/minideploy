package cmd

import (
	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Run build steps only",
	Long:  `Executes the build steps defined in .deploy.yml without uploading or deploying.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")
		if configPath == "" {
			var err error
			configPath, err = client.FindConfig()
			if err != nil {
				shared.Fatal("%v", err)
			}
		}

		cfg, err := client.LoadConfig(configPath)
		if err != nil {
			shared.Fatal("%v", err)
		}

		shared.Info("running build steps for %s", cfg.AppName)
		if err := client.RunBuildSteps(cfg.Build); err != nil {
			shared.Fatal("%v", err)
		}

		shared.Debug("verifying artifacts: %v", cfg.Artifacts)
		if err := client.VerifyArtifacts(cfg.Artifacts); err != nil {
			shared.Fatal("%v", err)
		}

		shared.Success("build complete for %s", cfg.AppName)
	},
}

func init() {
	rootCmd.AddCommand(buildCmd)
	buildCmd.Flags().StringP("config", "c", "", "path to .deploy.yml")
}
