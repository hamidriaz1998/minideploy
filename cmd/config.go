package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hamid/minideploy/internal/client"
	"github.com/hamid/minideploy/internal/shared"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage global minideploy configuration",
	Long:  `View and update the global configuration in ~/.config/minideploy/config.yml`,
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		cfg, err := client.LoadGlobalConfig()
		if err != nil {
			shared.Fatal("%v", err)
		}

		switch key {
		case "admin_key":
			if cfg.AdminKey == "" {
				fmt.Println("(not set)")
			} else {
				fmt.Println(cfg.AdminKey)
			}
		default:
			shared.Fatal("unknown config key %q", key)
		}
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key, value := args[0], args[1]
		cfg, err := client.LoadGlobalConfig()
		if err != nil {
			shared.Fatal("%v", err)
		}

		switch key {
		case "admin_key":
			cfg.AdminKey = value
		default:
			shared.Fatal("unknown config key %q", key)
		}

		if err := client.SaveGlobalConfig(cfg); err != nil {
			shared.Fatal("%v", err)
		}

		shared.Success("config %s updated", key)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
