package daemon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
