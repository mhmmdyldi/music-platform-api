package config

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// ParseEnvFile reads the KEY=VALUE format used by the files under config/.
//
// The format is deliberately the common subset that Docker Compose, the shell
// (`set -a; . file`) and this parser all read the same way: one assignment per
// line, blank lines and lines starting with # ignored, no quotes, no
// interpolation, no `export`.
func ParseEnvFile(r io.Reader) (map[string]string, error) {
	values := map[string]string{}
	scanner := bufio.NewScanner(r)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || key == "" || strings.ContainsAny(key, " \t") {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE, got %q", lineNo, line)
		}
		if strings.ContainsAny(value, `"'$`) {
			return nil, fmt.Errorf("line %d: %s: quotes and $ are not supported, write the value literally", lineNo, key)
		}
		if _, dup := values[key]; dup {
			return nil, fmt.Errorf("line %d: %s is set twice", lineNo, key)
		}
		values[key] = value
	}
	return values, scanner.Err()
}
