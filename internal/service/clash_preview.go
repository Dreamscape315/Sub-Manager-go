package service

import "strings"

// proxyLinePrefix is the standard two-space indented Clash proxy line prefix.
const proxyLinePrefix = "  - {"

// CountClashNodes counts proxy lines in a Clash YAML subscription body.
// Returns 0 for empty content or non-Clash targets (singbox/v2ray/surge/...);
// this is a deliberate rough parser, not a real YAML parser (see package docs
// in the original Java ClashPreview for the rationale).
func CountClashNodes(content string) int {
	if content == "" {
		return 0
	}
	count := 0
	for _, line := range splitLines(content) {
		if strings.HasPrefix(line, proxyLinePrefix) {
			count++
		}
	}
	return count
}

// ParseClashNodeNames extracts up to limit node names, in file order.
// Handles single/double quoted and bare `name:` values; unparsable lines are skipped.
func ParseClashNodeNames(content string, limit int) []string {
	var names []string
	if content == "" || limit <= 0 {
		return names
	}
	for _, line := range splitLines(content) {
		if !strings.HasPrefix(line, proxyLinePrefix) {
			continue
		}
		idx := strings.Index(line, "name:")
		if idx < 0 {
			continue
		}
		after := strings.TrimSpace(line[idx+len("name:"):])
		var name string
		if after != "" && (after[0] == '\'' || after[0] == '"') {
			quote := after[0]
			end := strings.IndexByte(after[1:], quote)
			if end < 0 {
				continue
			}
			name = after[1 : end+1]
		} else {
			end := strings.IndexByte(after, ',')
			if end < 0 {
				end = strings.IndexByte(after, '}')
			}
			if end < 0 {
				continue
			}
			name = strings.TrimSpace(after[:end])
		}
		names = append(names, name)
		if len(names) >= limit {
			break
		}
	}
	return names
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
