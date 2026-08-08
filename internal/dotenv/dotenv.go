package dotenv

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var expandPattern = regexp.MustCompile(`\$\{([^}]+)\}|\$([A-Za-z_][A-Za-z0-9_]*)`)

// Parse parses dotenv-format data into a map of key-value pairs.
// It supports comments (#), quoted values, export prefix, and variable expansion.
func Parse(data []byte) (map[string]string, error) {
	env := make(map[string]string)
	lines := bytes.Split(data, []byte("\n"))

	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Strip "export" prefix
		if strings.HasPrefix(trimmed, "export ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export"))
		}

		key, value, err := parseLine(trimmed)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum+1, err)
		}
		if key == "" {
			continue
		}

		// Expand variables using already-parsed values
		expanded := expandVars(value, env)
		env[key] = expanded
	}

	return env, nil
}

func parseLine(line string) (key, value string, err error) {
	// Find the first '='
	idx := strings.Index(line, "=")
	if idx < 0 {
		// Handle KEY without value (e.g., "KEY=")
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line), "", nil
		}
		return "", "", nil
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	if key == "" {
		return "", "", fmt.Errorf("empty key")
	}

	// Validate key format
	for i, r := range key {
		if i == 0 {
			if r != '_' && !isLetter(r) {
				return "", "", fmt.Errorf("invalid key %q: must start with letter or underscore", key)
			}
		} else if r != '_' && !isLetter(r) && !isDigit(r) {
			return "", "", fmt.Errorf("invalid key %q: must contain only letters, digits, underscores", key)
		}
	}

	// Handle quoted values
	quoted := false
	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			// Double-quoted: process escape sequences
			value = unquoteDouble(value[1 : len(value)-1])
			quoted = true
		} else if value[0] == '\'' && value[len(value)-1] == '\'' {
			// Single-quoted: raw value
			value = value[1 : len(value)-1]
			quoted = true
		}
	}

	// Strip inline comments (only for unquoted values)
	if !quoted {
		if commentIdx := strings.Index(value, " #"); commentIdx >= 0 {
			value = strings.TrimRight(value[:commentIdx], " ")
		}
	}

	return key, value, nil
}

func unquoteDouble(s string) string {
	var result strings.Builder
	result.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteByte('\n')
			case 't':
				result.WriteByte('\t')
			case 'r':
				result.WriteByte('\r')
			case '\\':
				result.WriteByte('\\')
			case '"':
				result.WriteByte('"')
			default:
				result.WriteByte(s[i])
				result.WriteByte(s[i+1])
			}
			i++
		} else {
			result.WriteByte(s[i])
		}
	}
	return result.String()
}

// expandVars replaces ${VAR} and $VAR references in value with values from env.
func expandVars(value string, env map[string]string) string {
	return expandPattern.ReplaceAllStringFunc(value, func(match string) string {
		var name string
		if match[0] == '$' && match[1] == '{' {
			// ${VAR}
			name = match[2 : len(match)-1]
		} else {
			// $VAR
			name = match[1:]
		}

		// Check parsed env first, then process env
		if val, ok := env[name]; ok {
			return val
		}
		if val, ok := os.LookupEnv(name); ok {
			return val
		}
		return match // Keep original if not found
	})
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// LoadFile reads and parses a .env file at the given path.
func LoadFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return Parse(data)
}

// LoadFromDir looks for a .env file in the given directory and loads it.
// Returns nil (not an error) if no .env file exists.
func LoadFromDir(dir string) (map[string]string, error) {
	envPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil, nil
	}
	return LoadFile(envPath)
}
