package web

import (
	"net/http"
	"net/url"
)

// setFlash stores a one-shot message in a short-lived cookie, read (and
// cleared) by popFlash on the very next request - equivalent to Spring's
// RedirectAttributes flash attributes but without a server-side session.
func setFlash(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    encodeCookie(value),
		Path:     "/",
		MaxAge:   10,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func flashOK(w http.ResponseWriter, msg string)  { setFlash(w, "flash_ok", msg) }
func flashErr(w http.ResponseWriter, msg string) { setFlash(w, "flash_err", msg) }

// popFlash reads and clears a flash cookie. Safe to call even if absent.
func popFlash(w http.ResponseWriter, r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil || c.Value == "" {
		return ""
	}
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1})
	return decodeCookie(c.Value)
}

func popFlashOK(w http.ResponseWriter, r *http.Request) string  { return popFlash(w, r, "flash_ok") }
func popFlashErr(w http.ResponseWriter, r *http.Request) string { return popFlash(w, r, "flash_err") }

func encodeCookie(s string) string { return url.QueryEscape(s) }
func decodeCookie(s string) string {
	v, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return v
}
