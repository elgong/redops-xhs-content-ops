package main

import (
	"os"
	"sort"
	"strings"
)

func updateDotEnv(path string, updates map[string]string) error {
	existing := map[string]string{}
	order := make([]string, 0)
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, "=") {
				continue
			}
			parts := strings.SplitN(trimmed, "=", 2)
			key := strings.TrimSpace(parts[0])
			if key == "" {
				continue
			}
			if _, ok := existing[key]; !ok {
				order = append(order, key)
			}
			existing[key] = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		}
	}
	for key, value := range updates {
		if _, ok := existing[key]; !ok {
			order = append(order, key)
		}
		existing[key] = value
	}
	seen := map[string]bool{}
	lines := make([]string, 0, len(existing))
	for _, key := range order {
		if _, ok := existing[key]; ok && !seen[key] {
			lines = append(lines, key+"="+existing[key])
			seen[key] = true
		}
	}
	rest := make([]string, 0)
	for key := range existing {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		lines = append(lines, key+"="+existing[key])
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
