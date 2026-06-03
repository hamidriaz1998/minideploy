package daemon

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hamid/minideploy/internal/shared"
)

func GenerateReleaseName() string {
	return time.Now().Format("20060102-150405")
}

func SnapshotRelease(deployPath, releaseName string) error {
	uploadDir := filepath.Join(deployPath, "upload")
	releaseDir := filepath.Join(deployPath, "releases", releaseName)

	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		return fmt.Errorf("upload directory does not exist: %s", uploadDir)
	}

	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return fmt.Errorf("create release dir: %w", err)
	}

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return fmt.Errorf("read upload dir: %w", err)
	}

	for _, entry := range entries {
		src := filepath.Join(uploadDir, entry.Name())
		dst := filepath.Join(releaseDir, entry.Name())

		if entry.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return fmt.Errorf("copy dir %s: %w", entry.Name(), err)
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("copy file %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

func UpdateSymlink(deployPath, releaseName string) (string, error) {
	currentLink := filepath.Join(deployPath, "current")
	releaseDir := filepath.Join("releases", releaseName)
	releaseTarget := filepath.Join(deployPath, releaseDir)

	previous := ""
	if existing, err := os.Readlink(currentLink); err == nil {
		previous = filepath.Base(existing)
	}

	tmp := currentLink + ".tmp"
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return previous, fmt.Errorf("remove tmp symlink: %w", err)
	}

	if err := os.Symlink(releaseTarget, tmp); err != nil {
		return previous, fmt.Errorf("create tmp symlink: %w", err)
	}

	if err := os.Rename(tmp, currentLink); err != nil {
		os.Remove(tmp)
		return previous, fmt.Errorf("atomic symlink swap: %w", err)
	}

	return previous, nil
}

func GetPreviousRelease(deployPath string) (string, error) {
	releasesDir := filepath.Join(deployPath, "releases")
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return "", fmt.Errorf("read releases dir: %w", err)
	}

	currentLink := filepath.Join(deployPath, "current")
	current, err := os.Readlink(currentLink)
	if err != nil {
		return "", fmt.Errorf("read current symlink: %w", err)
	}
	currentName := filepath.Base(current)

	var previous string
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != currentName {
			if entry.Name() > previous {
				previous = entry.Name()
			}
		}
	}

	if previous == "" {
		return "", fmt.Errorf("no previous release found")
	}
	return previous, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	fi, _ := in.Stat()
	if fi != nil {
		os.Chmod(dst, fi.Mode())
	}

	return out.Close()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target)
	})
}

func EnsureDeployDir(deployPath string) error {
	dirs := []string{
		filepath.Join(deployPath, "upload"),
		filepath.Join(deployPath, "releases"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return nil
}

func MakeRelease(releaseName string, app *shared.AppState) shared.Release {
	return shared.Release{
		Name:      releaseName,
		CreatedAt: time.Now(),
		IsCurrent: true,
	}
}

func CheckHealth(instances []shared.Instance, hc shared.HealthCheck) []shared.HealthResult {
	results := make([]shared.HealthResult, 0, len(instances))
	client := &http.Client{Timeout: time.Duration(hc.Timeout) * time.Second}

	for _, inst := range instances {
		result := shared.HealthResult{Instance: inst.ID, Port: inst.Port}
		url := fmt.Sprintf("http://localhost:%d%s", inst.Port, hc.Endpoint)

		for attempt := 0; attempt < hc.Retries; attempt++ {
			resp, err := client.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					result.Passed = true
					break
				}
				result.Error = fmt.Sprintf("status %d", resp.StatusCode)
			} else {
				result.Error = err.Error()
			}
			if attempt < hc.Retries-1 {
				time.Sleep(2 * time.Second)
			}
		}

		results = append(results, result)
		if hc.WaitBetweenInstances > 0 {
			time.Sleep(time.Duration(hc.WaitBetweenInstances) * time.Second)
		}
	}
	return results
}

func PruneReleases(deployPath string, releases []shared.Release, keep int) ([]string, error) {
	if keep <= 0 || len(releases) <= keep {
		return nil, nil
	}

	sorted := make([]shared.Release, len(releases))
	copy(sorted, releases)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	var pruned []string
	toRemove := len(sorted) - keep
	for i := 0; i < toRemove; i++ {
		name := sorted[i].Name
		releaseDir := filepath.Join(deployPath, "releases", name)
		os.RemoveAll(releaseDir)
		pruned = append(pruned, name)
	}
	return pruned, nil
}
