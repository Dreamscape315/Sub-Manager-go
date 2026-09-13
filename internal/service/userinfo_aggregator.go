package service

import (
	"strconv"
	"strings"
)

// AggregateUserinfo merges multiple Subscription-Userinfo header values into
// one: upload/download/total are summed, expire takes the earliest non-zero
// value. Returns ok=false if none of the inputs parsed to anything usable.
func AggregateUserinfo(headers []string) (string, bool) {
	var upload, download, total int64
	var minExpire int64
	haveExpire := false
	any := false

	for _, h := range headers {
		if strings.TrimSpace(h) == "" {
			continue
		}
		kv, ok := parseUserinfo(h)
		if !ok {
			continue
		}
		any = true
		upload += kv["upload"]
		download += kv["download"]
		total += kv["total"]
		if expire, present := kv["expire"]; present && expire > 0 {
			if !haveExpire || expire < minExpire {
				minExpire = expire
				haveExpire = true
			}
		}
	}
	if !any {
		return "", false
	}
	var sb strings.Builder
	sb.WriteString("upload=")
	sb.WriteString(strconv.FormatInt(upload, 10))
	sb.WriteString("; download=")
	sb.WriteString(strconv.FormatInt(download, 10))
	sb.WriteString("; total=")
	sb.WriteString(strconv.FormatInt(total, 10))
	if haveExpire {
		sb.WriteString("; expire=")
		sb.WriteString(strconv.FormatInt(minExpire, 10))
	}
	return sb.String(), true
}

func parseUserinfo(header string) (map[string]int64, bool) {
	kv := map[string]int64{}
	for _, segment := range strings.Split(header, ";") {
		trimmed := strings.TrimSpace(segment)
		if trimmed == "" {
			continue
		}
		eq := strings.IndexByte(trimmed, '=')
		if eq <= 0 || eq == len(trimmed)-1 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(trimmed[:eq]))
		value := strings.TrimSpace(trimmed[eq+1:])
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			kv[key] = n
		}
	}
	return kv, len(kv) > 0
}
