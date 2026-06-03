package cmd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"charm.land/huh/v2"
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

	var (
		appName             string
		serviceType         = "systemd"
		serviceName         string
		instanceCount       = "1"
		startPort           = "3000"
		deployPath          string
		buildSteps          string
		artifacts           string
		serverHost          string
		apiPort             = "8443"
		sshUser             = "root"
		apiKey              string
		envVars             string
		keepReleases        = "5"
		healthEndpoint      string
		healthTimeout       = "10"
		healthRetries       = "3"
		healthWaitInstances = "0"
	)

		appInput := huh.NewInput().
			Title("App name").
			Description("Name of your application").
			Value(&appName).
			Validate(func(s string) error {
				if s == "" {
					return errors.New("app name is required")
				}
				return nil
			})

		svcSelect := huh.NewSelect[string]().
			Title("Service type").
			Description("Process manager to use").
			Options(
				huh.NewOption("systemd", "systemd"),
				huh.NewOption("pm2", "pm2"),
			).
			Value(&serviceType)

		svcNameInput := huh.NewInput().
			Title("Service name").
			Description(`Use %i as a placeholder for the instance ID`).
			Value(&serviceName).
			Validate(func(s string) error {
				if s == "" {
					return errors.New("service name is required")
				}
				return nil
			})

		countInput := huh.NewInput().
			Title("Number of instances").
			Description("How many service instances to run").
			Value(&instanceCount).
			Validate(func(s string) error {
				n, err := strconv.Atoi(s)
				if err != nil || n < 1 {
					return errors.New("must be a positive number")
				}
				return nil
			})

		portInput := huh.NewInput().
			Title("Start port").
			Description("First instance port; subsequent instances increment by 1").
			Value(&startPort).
			Validate(func(s string) error {
				n, err := strconv.Atoi(s)
				if err != nil || n < 1 || n > 65535 {
					return errors.New("must be a valid port (1-65535)")
				}
				return nil
			})

		deployInput := huh.NewInput().
			Title("Deploy path").
			Description("Base path on the server").
			Value(&deployPath)

		buildInput := huh.NewText().
			Title("Build steps").
			Description("One command per line. At least one step required.").
			Value(&buildSteps).
			Validate(func(s string) error {
				if len(splitLines(s)) == 0 {
					return errors.New("at least one build step is required")
				}
				return nil
			})

		artifactInput := huh.NewText().
			Title("Artifacts").
			Description("Files/directories to upload. One per line.").
			Value(&artifacts).
			Validate(func(s string) error {
				if len(splitLines(s)) == 0 {
					return errors.New("at least one artifact is required")
				}
				return nil
			})

		hostInput := huh.NewInput().
			Title("Server host").
			Description("VPS hostname or IP address").
			Value(&serverHost).
			Validate(func(s string) error {
				if s == "" {
					return errors.New("server host is required")
				}
				return nil
			})

		portStrInput := huh.NewInput().
			Title("API port").
			Description("Daemon API port").
			Value(&apiPort)

		sshInput := huh.NewInput().
			Title("SSH user").
			Description("User for rsync/SSH connections").
			Value(&sshUser)

		keyInput := huh.NewInput().
			Title("API key").
			Description("Leave blank to use MINIDEPLOY_API_KEY env or .env file").
			Value(&apiKey)

		envInput := huh.NewText().
			Title("Environment variables").
			Description("Optional. KEY=VALUE, one per line. Leave empty to skip.").
			Value(&envVars)

		keepInput := huh.NewInput().
			Title("Keep releases").
			Description("Number of old releases to keep (0 = keep all)").
			Value(&keepReleases).
			Validate(func(s string) error {
				n, err := strconv.Atoi(s)
				if err != nil || n < 0 {
					return errors.New("must be a non-negative number")
				}
				return nil
			})

		healthEndpointInput := huh.NewInput().
			Title("Health check endpoint").
			Description("Optional. Path to check (e.g. /health). Leave empty to skip.").
			Value(&healthEndpoint)

		healthTimeoutInput := huh.NewInput().
			Title("Health check timeout (s)").
			Description("Seconds to wait per health check request").
			Value(&healthTimeout)

		healthRetriesInput := huh.NewInput().
			Title("Health check retries").
			Description("Number of retries per instance").
			Value(&healthRetries)

		healthWaitInput := huh.NewInput().
			Title("Wait between instances (s)").
			Description("Seconds to wait between checking each instance").
			Value(&healthWaitInstances)

		form := huh.NewForm(
			huh.NewGroup(
				appInput,
				svcSelect,
				svcNameInput,
				countInput,
				portInput,
			),
			huh.NewGroup(
				deployInput,
				buildInput,
				artifactInput,
			),
			huh.NewGroup(
				hostInput,
				portStrInput,
				sshInput,
				keyInput,
			),
			huh.NewGroup(
				envInput,
				keepInput,
			),
			huh.NewGroup(
				healthEndpointInput,
				healthTimeoutInput,
				healthRetriesInput,
				healthWaitInput,
			),
		)

		if err := form.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		count, _ := strconv.Atoi(instanceCount)
		port, _ := strconv.Atoi(startPort)
		if port == 0 {
			port = 3000
		}

		if appName == "" {
			appName = "my-app"
		}
		if serviceName == "" {
			serviceName = appName + "@%i"
		}
		if deployPath == "" {
			deployPath = "/opt/" + appName
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("app_name: %s\n", appName))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("service_type: %s\n", serviceType))
		b.WriteString(fmt.Sprintf("service_name: %s\n", serviceName))
		b.WriteString("\n")
		b.WriteString("instances:\n")
		for i := 0; i < count; i++ {
			p := port + i
			b.WriteString(fmt.Sprintf(`  - id: "%d"
    port: %d
    env:
      PORT: %d
`, p, p, p))
		}
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("deploy_path: %s\n", deployPath))
		b.WriteString("\n")
		b.WriteString("build:\n")
		for _, step := range splitLines(buildSteps) {
			b.WriteString(fmt.Sprintf("  - %s\n", step))
		}
		b.WriteString("\n")
		b.WriteString("artifacts:\n")
		for _, a := range splitLines(artifacts) {
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
		for _, line := range splitLines(envVars) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				b.WriteString(fmt.Sprintf("\nenv:\n  %s: %s\n", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])))
				break
			}
		}
		b.WriteString("\n")
		kr, _ := strconv.Atoi(keepReleases)
		if kr > 0 {
			b.WriteString(fmt.Sprintf("keep_releases: %d\n", kr))
		}
		if healthEndpoint != "" {
			ht, _ := strconv.Atoi(healthTimeout)
			if ht == 0 {
				ht = 10
			}
			hrCount, _ := strconv.Atoi(healthRetries)
			if hrCount == 0 {
				hrCount = 3
			}
			hwi, _ := strconv.Atoi(healthWaitInstances)
			b.WriteString("\nhealth_check:\n")
			b.WriteString(fmt.Sprintf("  endpoint: %s\n", healthEndpoint))
			b.WriteString(fmt.Sprintf("  timeout: %d\n", ht))
			b.WriteString(fmt.Sprintf("  retries: %d\n", hrCount))
			b.WriteString(fmt.Sprintf("  wait_between_instances: %d\n", hwi))
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

		fmt.Println("✅ .deploy.yml generated successfully!")
	},
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().BoolP("force", "f", false, "overwrite existing .deploy.yml")
}
