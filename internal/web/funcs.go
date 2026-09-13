package web

import (
	"fmt"
	"html/template"
	"time"
)

var funcMap = template.FuncMap{
	"bytesLabel":            bytesLabel,
	"containsID":            containsID,
	"containsStr":           containsStr,
	"dict":                  dict,
	"tsAttr":                tsAttr,
	"isZeroTime":            func(t time.Time) bool { return t.IsZero() },
	"add":                   func(a, b int) int { return a + b },
	"sub":                   func(a, b int) int { return a - b },
	"mapGetInt":             mapGetInt,
	"seq1":                  seq1,
	"standardTargetFormats": func() []string { return standardTargetFormats },
}

// standardTargetFormats populates the Profile target-format <select>; anything
// else typed in still works, it's just rendered as a "(自定义)" extra option.
var standardTargetFormats = []string{
	"clash", "clashr", "singbox", "surge", "quan", "quanx", "loon", "ss", "v2ray", "mixed",
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// bytesLabel formats a byte length like the Thymeleaf "bytes" fragment did:
// < 1KB -> "N B"; < 1MB -> "N.N KB"; else "N.NN MB". suffix is appended with a
// leading space when non-empty (e.g. bytesLabel(2048, "缓存") -> "2.0 KB 缓存").
func bytesLabel(n int, suffix string) string {
	var pretty string
	switch {
	case n < 1024:
		pretty = fmt.Sprintf("%d B", n)
	case n < 1048576:
		pretty = fmt.Sprintf("%.1f KB", float64(n)/1024.0)
	default:
		pretty = fmt.Sprintf("%.2f MB", float64(n)/1048576.0)
	}
	if suffix == "" {
		return pretty
	}
	return pretty + " " + suffix
}

// containsID reports whether id is present in ids ([]int64), used to render
// checkbox/badge state ("selected airport ids" membership checks).
func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// dict builds a map[string]any from alternating key/value args, letting
// templates pass multiple named values into a sub-template invocation.
func dict(pairs ...any) (map[string]any, error) {
	if len(pairs)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments")
	}
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, ok := pairs[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key %v is not a string", pairs[i])
		}
		m[key] = pairs[i+1]
	}
	return m, nil
}

// tsAttr formats a time.Time as RFC3339 for a data-ts="..." attribute that
// client-side JS (Alpine x-init) parses and renders in the browser's local time zone.
func tsAttr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func mapGetInt(m map[int64]int, key int64) int { return m[key] }

// seq1 returns [1, 2, ..., n] for use with {{range}} where a fixed-length
// index loop (rather than ranging over a slice) is convenient.
func seq1(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}
