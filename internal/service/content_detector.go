package service

import (
	"encoding/base64"
	"regexp"
	"strings"
)

// ContentKind is a rough guess at what shape a manually pasted subscription is.
type ContentKind int

const (
	KindUnknown ContentKind = iota
	KindClashYAML
	KindBase64
	KindURIList
)

func (k ContentKind) Label() string {
	switch k {
	case KindClashYAML:
		return "Clash YAML"
	case KindBase64:
		return "Base64"
	case KindURIList:
		return "URI 列表"
	default:
		return "未识别"
	}
}

// proxySchemes are the URI schemes subconverter recognizes as proxy nodes.
var proxySchemes = []string{
	"ss://", "ssr://", "vmess://", "vless://", "trojan://",
	"hysteria://", "hysteria2://", "hy2://", "tuic://", "snell://",
	"socks://", "socks5://", "http://", "https://",
}

var yamlKeys = []string{
	"proxies:", "proxy-providers:", "proxy-groups:", "mixed-port:",
	"socks-port:", "port:", "mode:", "rule-providers:",
}

var base64Charset = regexp.MustCompile(`^[A-Za-z0-9+/=_-]+$`)

// DetectContentKind mirrors SubscriptionContentDetector.detect: a lightweight,
// non-authoritative guess used only to give the admin UI quick feedback.
func DetectContentKind(content string) ContentKind {
	if strings.TrimSpace(content) == "" {
		return KindUnknown
	}
	trimmed := strings.TrimSpace(content)
	if looksLikeClashYAML(trimmed) {
		return KindClashYAML
	}
	if looksLikeURIList(trimmed) {
		return KindURIList
	}
	if looksLikeBase64(trimmed) {
		return KindBase64
	}
	return KindUnknown
}

func looksLikeClashYAML(s string) bool {
	lower := strings.ToLower(s)
	for _, key := range yamlKeys {
		if containsYAMLKeyAtLineStart(lower, key) {
			return true
		}
	}
	return false
}

// containsYAMLKeyAtLineStart requires the key to appear at the start of a line
// (ignoring leading whitespace), avoiding false positives like "# proxies:".
func containsYAMLKeyAtLineStart(lower, key string) bool {
	idx := 0
	for idx < len(lower) {
		pos := strings.Index(lower[idx:], key)
		if pos < 0 {
			return false
		}
		pos += idx
		lineStart := strings.LastIndexByte(lower[:pos], '\n') + 1
		prefix := lower[lineStart:pos]
		if strings.TrimSpace(prefix) == "" {
			return true
		}
		idx = pos + len(key)
	}
	return false
}

func looksLikeURIList(s string) bool {
	lines := regexp.MustCompile(`\r?\n`).Split(s, -1)
	matched := 0
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		isProxy := false
		lowerLine := strings.ToLower(line)
		for _, scheme := range proxySchemes {
			if strings.HasPrefix(lowerLine, scheme) {
				isProxy = true
				break
			}
		}
		if !isProxy {
			return false
		}
		matched++
	}
	return matched > 0
}

func looksLikeBase64(s string) bool {
	compact := regexp.MustCompile(`\s+`).ReplaceAllString(s, "")
	if len(compact) < 16 {
		return false
	}
	if !base64Charset.MatchString(compact) {
		return false
	}
	normalized := strings.NewReplacer("-", "+", "_", "/").Replace(compact)
	decoded, err := base64.StdEncoding.DecodeString(normalized)
	if err != nil {
		// Java's Base64 decoder tolerates missing padding better than Go's; retry raw encoding.
		decoded, err = base64.RawStdEncoding.DecodeString(normalized)
		if err != nil {
			return false
		}
	}
	lower := strings.ToLower(string(decoded))
	for _, scheme := range proxySchemes {
		if strings.Contains(lower, scheme) {
			return true
		}
	}
	return false
}
