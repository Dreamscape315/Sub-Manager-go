package web

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
	"github.com/akatsukisky/sub-manager-go/internal/service"
)

var slugPattern = regexp.MustCompile(domain.SlugPattern)

// reservedSlugs mirrors the Java controller's blacklist to avoid colliding
// with built-in routes.
var reservedSlugs = map[string]bool{
	"admin": true, "actuator": true, "error": true, "internal": true, "profile": true,
	"static": true, "webjars": true, "h2-console": true, "assets": true,
}

type profileForm struct {
	Slug               string
	TargetFormat       string
	ExternalConfig     string
	SelectedAirportIDs []int64
}

type profilesData struct {
	PageData
	Profiles    []domain.Profile
	AllAirports []domain.Airport
	EditingID   int64
	Form        profileForm
	Errors      map[string]string
}

// ProfilesList handles GET /admin/profiles.
func (d *Deps) ProfilesList(w http.ResponseWriter, r *http.Request) {
	profiles, err := d.Profiles.FindAllOrderByID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	airports, err := d.Airports.FindAllOrderByID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := profilesData{
		PageData:    d.pageData(w, r, "Profile 管理", "profiles"),
		Profiles:    profiles,
		AllAirports: airports,
		Form:        profileForm{TargetFormat: "clash"},
	}
	if editingID, ok := parseInt64(r.URL.Query().Get("editingId")); ok {
		for _, p := range profiles {
			if p.ID == editingID {
				data.EditingID = editingID
				data.Form = profileForm{
					Slug:               p.Slug,
					TargetFormat:       p.TargetFormat,
					ExternalConfig:     p.ExternalConfig,
					SelectedAirportIDs: p.SelectedAirportIDs,
				}
				break
			}
		}
	}
	render(w, "page:profiles", data)
}

func parseProfileForm(r *http.Request) profileForm {
	var ids []int64
	for _, raw := range r.Form["selectedAirportIds"] {
		if id, ok := parseInt64(raw); ok {
			ids = append(ids, id)
		}
	}
	return profileForm{
		Slug:               strings.TrimSpace(r.FormValue("slug")),
		TargetFormat:       strings.TrimSpace(r.FormValue("targetFormat")),
		ExternalConfig:     strings.TrimSpace(r.FormValue("externalConfig")),
		SelectedAirportIDs: ids,
	}
}

// validateProfileForm applies field-level checks plus the reserved-word /
// uniqueness rule that needs DB access (excludeID = 0 for create).
func (d *Deps) validateProfileForm(f profileForm, excludeID int64) map[string]string {
	errs := map[string]string{}
	normalized := strings.ToLower(f.Slug)
	if f.Slug == "" {
		errs["slug"] = "slug 不能为空"
	} else if !slugPattern.MatchString(normalized) {
		errs["slug"] = "slug 只能包含小写字母、数字、短横线，长度 1-64"
	} else if reservedSlugs[normalized] {
		errs["slug"] = "该 slug 是保留字，请换一个"
	} else if exists, _ := d.Profiles.ExistsBySlugExcludingID(normalized, excludeID); exists {
		errs["slug"] = "slug 已被占用"
	}
	if f.TargetFormat == "" {
		errs["targetFormat"] = "目标格式不能为空"
	} else if len(f.TargetFormat) > 32 {
		errs["targetFormat"] = "目标格式最长 32 字符"
	}
	if len(f.ExternalConfig) > 2048 {
		errs["externalConfig"] = "external config 最长 2048 字符"
	}
	return errs
}

func applyProfileForm(p *domain.Profile, f profileForm) {
	p.Slug = strings.ToLower(f.Slug)
	p.TargetFormat = f.TargetFormat
	p.ExternalConfig = f.ExternalConfig
	p.SelectedAirportIDs = f.SelectedAirportIDs
}

// ProfilesCreate handles POST /admin/profiles.
func (d *Deps) ProfilesCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	form := parseProfileForm(r)
	errs := d.validateProfileForm(form, 0)
	if len(errs) > 0 {
		d.rerenderProfilesWithErrors(w, r, form, errs, 0)
		return
	}
	profile := domain.Profile{Enabled: true}
	applyProfileForm(&profile, form)
	if err := d.Profiles.Insert(&profile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	flashOK(w, "已新增 Profile："+profile.Slug)
	http.Redirect(w, r, "/admin/profiles", http.StatusFound)
}

// ProfilesUpdate handles POST /admin/profiles/{id}.
func (d *Deps) ProfilesUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	profile, err := d.Profiles.FindByID(id)
	if err != nil || profile == nil {
		flashErr(w, "Profile 不存在 (id="+strconv.FormatInt(id, 10)+")")
		http.Redirect(w, r, "/admin/profiles", http.StatusFound)
		return
	}
	r.ParseForm()
	form := parseProfileForm(r)
	errs := d.validateProfileForm(form, id)
	if len(errs) > 0 {
		d.rerenderProfilesWithErrors(w, r, form, errs, id)
		return
	}
	applyProfileForm(profile, form)
	if err := d.Profiles.Update(profile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	flashOK(w, "已保存："+profile.Slug)
	http.Redirect(w, r, "/admin/profiles", http.StatusFound)
}

func (d *Deps) rerenderProfilesWithErrors(w http.ResponseWriter, r *http.Request, form profileForm, errs map[string]string, editingID int64) {
	profiles, _ := d.Profiles.FindAllOrderByID()
	airports, _ := d.Airports.FindAllOrderByID()
	render(w, "page:profiles", profilesData{
		PageData:    d.pageData(w, r, "Profile 管理", "profiles"),
		Profiles:    profiles,
		AllAirports: airports,
		EditingID:   editingID,
		Form:        form,
		Errors:      errs,
	})
}

// ProfilesDelete handles POST /admin/profiles/{id}/delete.
func (d *Deps) ProfilesDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	profile, _ := d.Profiles.FindByID(id)
	if profile != nil {
		d.Profiles.Delete(id)
		flashOK(w, "已删除："+profile.Slug)
	}
	http.Redirect(w, r, "/admin/profiles", http.StatusFound)
}

// ProfilesToggle handles POST /admin/profiles/{id}/toggle.
func (d *Deps) ProfilesToggle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	profile, _ := d.Profiles.FindByID(id)
	if profile != nil {
		newEnabled := !profile.Enabled
		d.Profiles.SetEnabled(id, newEnabled)
		verb := "已停用："
		if newEnabled {
			verb = "已启用："
		}
		flashOK(w, verb+profile.Slug)
	}
	http.Redirect(w, r, "/admin/profiles", http.StatusFound)
}

// ProfilesSynthesizeNow handles POST /admin/profiles/{id}/synthesize.
func (d *Deps) ProfilesSynthesizeNow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	success := d.Synthesizer.Synthesize(id)
	profile, _ := d.Profiles.FindByID(id)
	if profile != nil {
		if success {
			flashOK(w, "已合成："+profile.Slug)
		} else {
			flashErr(w, "合成失败："+profile.Slug+"（详见后端日志；旧内容保留）")
		}
	}
	http.Redirect(w, r, "/admin/profiles", http.StatusFound)
}

type profilePreviewData struct {
	Profile   *domain.Profile
	NodeNames []string
}

// ProfilesPreview handles GET /admin/profiles/{id}/preview: an htmx fragment
// listing up to 100 node names parsed from the last synthesized Clash YAML.
func (d *Deps) ProfilesPreview(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	profile, _ := d.Profiles.FindByID(id)
	var names []string
	if profile != nil {
		names = service.ParseClashNodeNames(profile.CachedContent, 100)
	}
	renderFragment(w, "fragment:profile-preview", profilePreviewData{Profile: profile, NodeNames: names})
}
