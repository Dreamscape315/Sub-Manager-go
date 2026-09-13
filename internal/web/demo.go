package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DemoAirport handles GET /demo/airport/{name}: a fake airport endpoint so
// "multi-source merge" can be tested without real airports. Gated by
// Deps.DemoEnabled (mirrors submanager.demo.enabled). Every node points at
// 127.0.0.1:443 and never actually connects - it only exercises the
// synthesize/aggregate pipeline.
func (d *Deps) DemoAirport(w http.ResponseWriter, r *http.Request) {
	if !d.DemoEnabled {
		http.NotFound(w, r)
		return
	}
	name := r.PathValue("name")
	count := 3
	if raw := r.URL.Query().Get("count"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			count = n
		}
	}
	if count < 1 {
		count = 1
	}
	if count > 50 {
		count = 50
	}

	var body strings.Builder
	fmt.Fprintf(&body, "# demo airport: %s\n", name)
	body.WriteString("proxies:\n")
	for i := 1; i <= count; i++ {
		fmt.Fprintf(&body,
			"  - {name: \"demo-%s-%d\", type: trojan, server: 127.0.0.1, port: 443, "+
				"password: \"demo-pw-%s-%d\", sni: example.com, skip-cert-verify: true, udp: true}\n",
			name, i, name, i)
	}

	total := int64(100) * 1024 * 1024 * 1024
	expire := time.Now().Add(30 * 24 * time.Hour).Unix()
	userinfo := fmt.Sprintf("upload=0; download=0; total=%d; expire=%d", total, expire)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Subscription-Userinfo", userinfo)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(body.String()))
}
