package client

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func RunBuildSteps(steps []string) error {
	for i, step := range steps {
		fmt.Fprintf(os.Stdout, "[build] (%d/%d) %s\n", i+1, len(steps), step)

		cmd := exec.Command("sh", "-c", step)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = os.Environ()

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("build step %d failed: %s: %w", i+1, step, err)
		}
	}

	fmt.Fprintf(os.Stdout, "[build] all %d steps completed\n", len(steps))
	return nil
}

func VerifyArtifacts(artifacts []string) error {
	var missing []string
	for _, a := range artifacts {
		if _, err := os.Stat(a); os.IsNotExist(err) {
			missing = append(missing, a)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing artifacts after build: %s", strings.Join(missing, ", "))
	}
	return nil
}
