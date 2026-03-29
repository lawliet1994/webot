package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadDotEnv loads KEY=VALUE pairs from a .env file into the current process.
// Existing environment variables are preserved and not overridden.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				unquoted, err := strconv.Unquote(`"` + strings.Trim(value, `"'`) + `"`)
				if err == nil {
					value = unquoted
				} else {
					value = strings.Trim(value, `"'`)
				}
			}
		}

		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set env %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan .env: %w", err)
	}
	return nil
}
