package shared

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type SSHHostEntry struct {
	HostName string
	User     string
	Port     string
	IdentityFile string
}

func ParseSSHConfig() (map[string]SSHHostEntry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}

	path := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open ~/.ssh/config: %w", err)
	}
	defer f.Close()

	entries := make(map[string]SSHHostEntry)
	var currentHost string
	var currentEntry SSHHostEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(strings.ToLower(line), "host ") {
			if currentHost != "" {
				entries[currentHost] = currentEntry
			}
			currentHost = strings.TrimSpace(strings.TrimPrefix(line, "host "))
			currentHost = strings.TrimSpace(strings.TrimPrefix(currentHost, "Host "))
			currentEntry = SSHHostEntry{}
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "hostname":
			currentEntry.HostName = value
		case "user":
			currentEntry.User = value
		case "port":
			currentEntry.Port = value
		case "identityfile":
			currentEntry.IdentityFile = value
		}
	}

	if currentHost != "" {
		entries[currentHost] = currentEntry
	}

	return entries, scanner.Err()
}

func ResolveSSHHost(host string) (string, string, error) {
	entries, err := ParseSSHConfig()
	if err != nil {
		return host, "", fmt.Errorf("parse ssh config: %w", err)
	}

	entry, ok := entries[host]
	if !ok {
		return host, "", nil
	}

	resolved := host
	if entry.HostName != "" {
		resolved = entry.HostName
	}
	return resolved, entry.User, nil
}
