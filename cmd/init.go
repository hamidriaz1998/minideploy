package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a .deploy.yml in the current directory",
	Long: `Interactively generates a .deploy.yml configuration file
in the current directory. All prompts have sensible defaults
so you can just press Enter to accept them.

Use 'minideploy init --force' to overwrite an existing file.`,
	Run: func(cmd *cobra.Command, args []string) {
		force, _ := cmd.Flags().GetBool("force")

		if _, err := os.Stat(".deploy.yml"); err == nil && !force {
			fmt.Fprintln(os.Stderr, ".deploy.yml already exists. Use --force to overwrite.")
			os.Exit(1)
		}

		reader := bufio.NewReader(os.Stdin)

		prompt := func(label, defaultVal string) string {
			if defaultVal != "" {
				fmt.Printf("%s [%s]: ", label, defaultVal)
			} else {
				fmt.Printf("%s: ", label)
			}
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				return defaultVal
			}
			return input
		}

		fmt.Println("minideploy init — generating .deploy.yml")
		fmt.Println(strings.Repeat("-", 40))

		appName := prompt("App name", "my-app")
		serviceType := prompt("Service type (systemd/pm2)", "systemd")
		serviceName := fmt.Sprintf("%s@%%i", appName)
		serviceName = prompt("Service name (use %i for instances)", serviceName)

		instanceCountStr := prompt("Number of instances", "1")
		instanceCount, _ := strconv.Atoi(instanceCountStr)
		if instanceCount < 1 {
			instanceCount = 1
		}

		instances := make([]string, 0, instanceCount)
		for i := 1; i <= instanceCount; i++ {
			portStr := prompt(fmt.Sprintf("  Instance %d port", i), fmt.Sprintf("%d", 3000+i-1))
			port, _ := strconv.Atoi(portStr)
			if port == 0 {
				port = 3000 + i - 1
			}
			instances = append(instances, fmt.Sprintf(`  - id: "%d"
    port: %d
    env:
      PORT: %d`, port, port, port))
		}

		deployPath := prompt("Deploy path", fmt.Sprintf("/opt/%s", appName))

		fmt.Println("Build steps (one per line, empty line to finish):")
		var buildSteps []string
		for {
			step := prompt(fmt.Sprintf("  Step %d", len(buildSteps)+1), "")
			if step == "" {
				if len(buildSteps) == 0 {
					fmt.Println("  (at least one build step is required)")
					continue
				}
				break
			}
			buildSteps = append(buildSteps, step)
		}

		fmt.Println("Artifacts to upload (one per line, empty line to finish):")
		var artifacts []string
		for {
			artifact := prompt(fmt.Sprintf("  Artifact %d", len(artifacts)+1), "")
			if artifact == "" {
				if len(artifacts) == 0 {
					fmt.Println("  (at least one artifact is required)")
					continue
				}
				break
			}
			artifacts = append(artifacts, artifact)
		}

		serverHost := prompt("Server host", "")
		for serverHost == "" {
			serverHost = prompt("Server host", "")
		}
		apiPort := prompt("Server API port", "8443")
		sshUser := prompt("SSH user", "root")
		apiKey := prompt("API key (leave blank for env/MINIDEPLOY_API_KEY)", "")

		var envVars []string
		fmt.Println("Environment variables (KEY=VALUE, one per line, empty to finish):")
		for {
			env := prompt(fmt.Sprintf("  Env %d", len(envVars)+1), "")
			if env == "" {
				break
			}
			envVars = append(envVars, env)
		}

		// Write the file
		var b strings.Builder
		b.WriteString(fmt.Sprintf("app_name: %s\n", appName))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("service_type: %s\n", serviceType))
		b.WriteString(fmt.Sprintf("service_name: %s\n", serviceName))
		b.WriteString("\n")
		b.WriteString("instances:\n")
		for _, inst := range instances {
			b.WriteString(inst + "\n")
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("deploy_path: %s\n", deployPath))
		b.WriteString("\n")
		b.WriteString("build:\n")
		for _, step := range buildSteps {
			b.WriteString(fmt.Sprintf("  - %s\n", step))
		}
		b.WriteString("\n")
		b.WriteString("artifacts:\n")
		for _, a := range artifacts {
			b.WriteString(fmt.Sprintf("  - %s\n", a))
		}
		b.WriteString("\n")
		b.WriteString("server:\n")
		b.WriteString(fmt.Sprintf("  host: %s\n", serverHost))
		b.WriteString(fmt.Sprintf("  api_port: %s\n", apiPort))
		b.WriteString(fmt.Sprintf("  ssh_user: %s\n", sshUser))
		if apiKey != "" {
			b.WriteString(fmt.Sprintf("  api_key: %s\n", apiKey))
		} else {
			b.WriteString("  # api_key: <set MINIDEPLOY_API_KEY env or add here>\n")
		}
		if len(envVars) > 0 {
			b.WriteString("\n")
			b.WriteString("env:\n")
			for _, e := range envVars {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) == 2 {
					b.WriteString(fmt.Sprintf("  %s: %s\n", parts[0], parts[1]))
				}
			}
		}
		b.WriteString("\n")
		b.WriteString("# pre_deploy:\n")
		b.WriteString("#   - cmd: make migrate\n")
		b.WriteString("# post_deploy:\n")
		b.WriteString("#   - cmd: make warmup\n")

		if err := os.WriteFile(".deploy.yml", []byte(b.String()), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error writing .deploy.yml: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(strings.Repeat("-", 40))
		fmt.Println(".deploy.yml generated successfully!")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolP("force", "f", false, "overwrite existing .deploy.yml")
}
