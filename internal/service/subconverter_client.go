package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SubconverterClient calls the subconverter /sub endpoint. Each call builds a
// fresh *http.Client from the latest AppSettings (base URL + timeout), so
// changing them in /admin/settings takes effect on the very next call without
// a restart.
type SubconverterClient struct {
	settings *SettingsService
}

func NewSubconverterClient(settings *SettingsService) *SubconverterClient {
	return &SubconverterClient{settings: settings}
}

type ConversionResult struct {
	Body        string
	ContentType string
}

type SubconverterError struct {
	msg string
}

func (e *SubconverterError) Error() string { return e.msg }

// Convert calls subconverter's /sub?target=...&url=...&config=....
func (c *SubconverterClient) Convert(target, urlList, externalConfig string) (ConversionResult, error) {
	s := c.settings.Get()
	client := &http.Client{Timeout: time.Duration(s.SubconverterTimeoutSeconds) * time.Second}

	q := url.Values{}
	q.Set("target", target)
	q.Set("url", urlList)
	if strings.TrimSpace(externalConfig) != "" {
		q.Set("config", externalConfig)
	}
	base := strings.TrimRight(s.SubconverterBaseURL, "/")
	reqURL := base + "/sub?" + q.Encode()

	started := time.Now()
	resp, err := client.Get(reqURL)
	if err != nil {
		cost := time.Since(started).Milliseconds()
		return ConversionResult{}, &SubconverterError{
			msg: fmt.Sprintf("subconverter call failed after %dms: %s", cost, err.Error()),
		}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	cost := time.Since(started).Milliseconds()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := fmt.Sprintf("subconverter call failed after %dms: HTTP %d", cost, resp.StatusCode)
		if hint := classifyHint(string(body)); hint != "" {
			msg += "  [hint] " + hint
		}
		return ConversionResult{}, &SubconverterError{msg: msg}
	}
	if len(body) == 0 {
		return ConversionResult{}, &SubconverterError{
			msg: fmt.Sprintf("subconverter returned empty body (cost %dms)", cost),
		}
	}
	return ConversionResult{Body: string(body), ContentType: resp.Header.Get("Content-Type")}, nil
}

// classifyHint maps the common "No nodes were found" subconverter error to an
// actionable hint. Kept for parity with the Java client's diagnostics.
func classifyHint(body string) string {
	if strings.Contains(body, "No nodes were found") {
		return "check /admin/settings 的 internalBaseUrl 是否 subconverter 容器可达"
	}
	return ""
}
