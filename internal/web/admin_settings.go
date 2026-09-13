package web

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var httpURLOrBlankPattern = regexp.MustCompile(`^(|https?://\S+)$`)

type settingsForm struct {
	RefreshIntervalSeconds     int
	SubconverterTimeoutSeconds int
	AirportFetchTimeoutSeconds int
	SubconverterBaseURL        string
	InternalBaseURL            string
	PublicBaseURL              string
	InternalSecret             string
}

type settingsData struct {
	PageData
	Form   settingsForm
	Errors map[string]string
}

// SettingsView handles GET /admin/settings.
func (d *Deps) SettingsView(w http.ResponseWriter, r *http.Request) {
	s := d.Settings.Get()
	render(w, "page:settings", settingsData{
		PageData: d.pageData(w, r, "系统设置", "settings"),
		Form: settingsForm{
			RefreshIntervalSeconds:      s.RefreshIntervalSeconds,
			SubconverterTimeoutSeconds:  s.SubconverterTimeoutSeconds,
			AirportFetchTimeoutSeconds:  s.AirportFetchTimeoutSeconds,
			SubconverterBaseURL:         s.SubconverterBaseURL,
			InternalBaseURL:             s.InternalBaseURL,
			PublicBaseURL:               s.PublicBaseURL,
			InternalSecret:              s.InternalSecret,
		},
	})
}

func parseSettingsForm(r *http.Request) settingsForm {
	atoi := func(name string) int {
		n, _ := strconv.Atoi(r.FormValue(name))
		return n
	}
	return settingsForm{
		RefreshIntervalSeconds:     atoi("refreshIntervalSeconds"),
		SubconverterTimeoutSeconds: atoi("subconverterTimeoutSeconds"),
		AirportFetchTimeoutSeconds: atoi("airportFetchTimeoutSeconds"),
		SubconverterBaseURL:        strings.TrimSpace(r.FormValue("subconverterBaseUrl")),
		InternalBaseURL:            strings.TrimSpace(r.FormValue("internalBaseUrl")),
		PublicBaseURL:              strings.TrimSpace(r.FormValue("publicBaseUrl")),
		InternalSecret:             strings.TrimSpace(r.FormValue("internalSecret")),
	}
}

func validateSettingsForm(f settingsForm) map[string]string {
	errs := map[string]string{}
	if f.RefreshIntervalSeconds < 30 || f.RefreshIntervalSeconds > 86400 {
		errs["refreshIntervalSeconds"] = "刷新间隔必须在 30 - 86400 秒之间"
	}
	if f.SubconverterTimeoutSeconds < 1 || f.SubconverterTimeoutSeconds > 600 {
		errs["subconverterTimeoutSeconds"] = "必须在 1 - 600 秒之间"
	}
	if f.AirportFetchTimeoutSeconds < 1 || f.AirportFetchTimeoutSeconds > 600 {
		errs["airportFetchTimeoutSeconds"] = "必须在 1 - 600 秒之间"
	}
	if f.SubconverterBaseURL == "" || !httpURLPattern.MatchString(f.SubconverterBaseURL) {
		errs["subconverterBaseUrl"] = "必须以 http:// 或 https:// 开头，且不能包含空白字符"
	}
	if f.InternalBaseURL == "" || !httpURLPattern.MatchString(f.InternalBaseURL) {
		errs["internalBaseUrl"] = "必须以 http:// 或 https:// 开头，且不能包含空白字符"
	}
	if !httpURLOrBlankPattern.MatchString(f.PublicBaseURL) {
		errs["publicBaseUrl"] = "必须以 http:// 或 https:// 开头，或留空"
	}
	if len(f.InternalSecret) > 128 {
		errs["internalSecret"] = "最长 128 字符"
	}
	return errs
}

// normalizeBaseURL trims whitespace and trailing slashes so saved settings and
// generated test URLs never produce a double slash like "https://x.com//foo".
func normalizeBaseURL(s string) string {
	out := strings.TrimSpace(s)
	for strings.HasSuffix(out, "/") {
		out = out[:len(out)-1]
	}
	return out
}

// SettingsSave handles POST /admin/settings.
func (d *Deps) SettingsSave(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	form := parseSettingsForm(r)
	errs := validateSettingsForm(form)
	if len(errs) > 0 {
		render(w, "page:settings", settingsData{
			PageData: d.pageData(w, r, "系统设置", "settings"),
			Form:     form,
			Errors:   errs,
		})
		return
	}
	current := d.Settings.Get()
	current.RefreshIntervalSeconds = form.RefreshIntervalSeconds
	current.SubconverterTimeoutSeconds = form.SubconverterTimeoutSeconds
	current.AirportFetchTimeoutSeconds = form.AirportFetchTimeoutSeconds
	current.SubconverterBaseURL = normalizeBaseURL(form.SubconverterBaseURL)
	current.InternalBaseURL = normalizeBaseURL(form.InternalBaseURL)
	current.PublicBaseURL = normalizeBaseURL(form.PublicBaseURL)
	current.InternalSecret = form.InternalSecret
	if _, err := d.Settings.Save(current); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	flashOK(w, "已保存。下一次调度/调用即生效，无需重启。")
	http.Redirect(w, r, "/admin/settings", http.StatusFound)
}

type connTestResult struct {
	Label   string
	URL     string
	OK      bool
	Message string
	CostMs  int64
	Skipped bool
}

// SettingsTestConnectivity handles POST /admin/settings/test-connectivity: an
// htmx-triggered three-way probe (app->subconverter, app->self via localhost,
// subconverter->app via internalBaseUrl) to diagnose the most common
// "No nodes were found" misconfiguration.
func (d *Deps) SettingsTestConnectivity(w http.ResponseWriter, r *http.Request) {
	s := d.Settings.Get()
	var results []connTestResult

	results = append(results, d.hit("app → subconverter (/version)",
		normalizeBaseURL(s.SubconverterBaseURL)+"/version",
		func(body string) string {
			if body == "" {
				return "响应为空"
			}
			return "版本：" + strings.TrimSpace(body)
		}))

	airports, _ := d.Airports.FindEnabledOrderByID()
	tokenQuery := buildTokenQuery(s.InternalSecret)
	selfBase := fmt.Sprintf("http://localhost:%d", d.ServerPort)
	if len(airports) == 0 {
		results = append(results, connTestResult{Label: "app → 本地 /internal 端点（自回环）", Skipped: true,
			Message: "跳过（没有 enabled 机场作为测试目标）"})
	} else {
		first := airports[0]
		u := fmt.Sprintf("%s/internal/airports/%d/raw%s", selfBase, first.ID, tokenQuery)
		results = append(results, d.hit("app → 本地 /internal 端点（自回环，走 localhost）", u,
			func(body string) string { return fmt.Sprintf("拿到 %d 字节", len(body)) }))
	}

	if len(airports) == 0 {
		results = append(results, connTestResult{Label: "subconverter → 本平台 /internal（真实链路）", Skipped: true,
			Message: "跳过（没有 enabled 机场作为测试目标）"})
	} else {
		first := airports[0]
		if !first.HasCachedContent() {
			results = append(results, connTestResult{Label: "subconverter → 本平台 /internal（真实链路）", Skipped: true,
				Message: fmt.Sprintf("跳过（机场 #%d 还没有缓存，请先点它的『立即拉取』）", first.ID)})
		} else {
			innerURL := fmt.Sprintf("%s/internal/airports/%d/raw%s", normalizeBaseURL(s.InternalBaseURL), first.ID, tokenQuery)
			subURL := normalizeBaseURL(s.SubconverterBaseURL) + "/sub?target=clash&url=" + url.QueryEscape(innerURL)
			results = append(results, d.hit("subconverter → 本平台 /internal（真实链路，走 internalBaseUrl）", subURL,
				func(body string) string {
					if body == "" {
						return "响应为空"
					}
					if strings.Contains(body, "No nodes were found") {
						return "❌ subconverter 拿不到内部端点内容（大概率 internalBaseUrl 在 subconverter 容器视角不可达，或 internalSecret 不匹配）"
					}
					nodes := 0
					for _, line := range strings.Split(body, "\n") {
						if strings.HasPrefix(line, "  - {") {
							nodes++
						}
					}
					return fmt.Sprintf("✅ 链路通畅：subconverter 从机场 #%d 拉到 %d 个节点（仅本机场裸转换，无 externalConfig 过滤；"+
						"profile 合成的总节点数会因参与机场数、externalConfig 过滤等不同）", first.ID, nodes)
				}))
		}
	}

	renderFragment(w, "fragment:connectivity-result", struct{ Results []connTestResult }{Results: results})
}

func buildTokenQuery(secret string) string {
	if strings.TrimSpace(secret) == "" {
		return ""
	}
	return "?token=" + url.QueryEscape(secret)
}

func (d *Deps) hit(label, u string, summarize func(body string) string) connTestResult {
	client := &http.Client{Timeout: 10 * time.Second}
	start := time.Now()
	resp, err := client.Get(u)
	cost := time.Since(start).Milliseconds()
	if err != nil {
		return connTestResult{Label: label, URL: u, OK: false, CostMs: cost,
			Message: fmt.Sprintf("%T: %s", err, safeTrim(err.Error()))}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	body := string(bodyBytes)
	ok := resp.StatusCode >= 200 && resp.StatusCode < 300
	msg := summarize(body)
	if !ok {
		msg = fmt.Sprintf("HTTP %d %s", resp.StatusCode, msg)
	}
	return connTestResult{Label: label, URL: u, OK: ok, CostMs: cost, Message: msg}
}

func safeTrim(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

// RefreshAll handles POST /admin/refresh: the dashboard's "refresh everything now" button.
func (d *Deps) RefreshAll(w http.ResponseWriter, r *http.Request) {
	result := d.Orchestrator.RefreshAll()
	if !result.Executed {
		flashErr(w, "另一轮刷新正在进行中（定时任务或另一次手动点击），本次跳过。稍后再看结果。")
	} else {
		flashOK(w, fmt.Sprintf("已刷新：机场 %d/%d 成功，Profile %d/%d 成功，用时 %ds",
			result.OKAirports, result.TotalAirports, result.OKProfiles, result.TotalProfiles,
			int(result.Duration.Seconds())))
	}
	http.Redirect(w, r, "/admin", http.StatusFound)
}
