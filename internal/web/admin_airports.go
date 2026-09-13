package web

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
	"github.com/akatsukisky/sub-manager-go/internal/service"
)

var httpURLPattern = regexp.MustCompile(`^https?://\S+$`)

type airportForm struct {
	Name          string
	SourceType    string // "URL" or "MANUAL"
	SubURL        string
	ManualContent string
}

func (f airportForm) isManual() bool { return f.SourceType == "MANUAL" }

type airportsData struct {
	PageData
	Airports  []domain.Airport
	UsedByMap map[int64][]domain.Profile
	EditingID int64
	Form      airportForm
	Errors    map[string]string
}

// AirportsList handles GET /admin/airports (list + optional inline edit form).
func (d *Deps) AirportsList(w http.ResponseWriter, r *http.Request) {
	airports, err := d.Airports.FindAllOrderByID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	profiles, err := d.Profiles.FindAllOrderByID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := airportsData{
		PageData:  d.pageData(w, r, "机场管理", "airports"),
		Airports:  airports,
		UsedByMap: buildUsedByMap(airports, profiles),
		Form:      airportForm{SourceType: "URL"},
	}
	if editingID, ok := parseInt64(r.URL.Query().Get("editingId")); ok {
		for _, a := range airports {
			if a.ID == editingID {
				data.EditingID = editingID
				data.Form = formFromAirport(a)
				break
			}
		}
	}
	render(w, "page:airports", data)
}

func formFromAirport(a domain.Airport) airportForm {
	if a.IsManualSource() {
		return airportForm{Name: a.Name, SourceType: "MANUAL", ManualContent: a.ManualContent}
	}
	return airportForm{Name: a.Name, SourceType: "URL", SubURL: a.SubURL}
}

// buildUsedByMap computes, for each airport, which profiles would include it -
// empty SelectedAirportIDs means "all enabled airports", so those profiles
// count for every currently-enabled airport.
func buildUsedByMap(airports []domain.Airport, profiles []domain.Profile) map[int64][]domain.Profile {
	result := make(map[int64][]domain.Profile, len(airports))
	for _, a := range airports {
		var using []domain.Profile
		for _, p := range profiles {
			if len(p.SelectedAirportIDs) == 0 {
				if a.Enabled {
					using = append(using, p)
				}
				continue
			}
			for _, id := range p.SelectedAirportIDs {
				if id == a.ID {
					using = append(using, p)
					break
				}
			}
		}
		result[a.ID] = using
	}
	return result
}

func parseAirportForm(r *http.Request) airportForm {
	return airportForm{
		Name:          strings.TrimSpace(r.FormValue("name")),
		SourceType:    r.FormValue("sourceType"),
		SubURL:        strings.TrimSpace(r.FormValue("subUrl")),
		ManualContent: r.FormValue("manualContent"),
	}
}

func validateAirportForm(f airportForm) map[string]string {
	errs := map[string]string{}
	if f.Name == "" {
		errs["name"] = "名称不能为空"
	} else if len(f.Name) > 128 {
		errs["name"] = "名称最长 128 字符"
	}
	if f.isManual() {
		if strings.TrimSpace(f.ManualContent) == "" {
			errs["manualContent"] = "手动模式下请粘贴订阅内容"
		} else if len(f.ManualContent) > 2*1024*1024 {
			errs["manualContent"] = "内容太大，最多 2 MB"
		}
	} else {
		if f.SubURL == "" {
			errs["subUrl"] = "URL 模式下订阅链接不能为空"
		} else if !httpURLPattern.MatchString(f.SubURL) {
			errs["subUrl"] = "必须以 http:// 或 https:// 开头，且不能包含空白字符"
		} else if len(f.SubURL) > 2048 {
			errs["subUrl"] = "订阅 URL 最长 2048 字符"
		}
	}
	return errs
}

func applyAirportForm(a *domain.Airport, f airportForm) {
	a.Name = f.Name
	if f.isManual() {
		a.SourceType = "MANUAL"
		a.ManualContent = f.ManualContent
		slug := strconv.FormatInt(a.ID, 10)
		a.SubURL = "manual://airport-" + slug
	} else {
		a.SourceType = "URL"
		a.SubURL = f.SubURL
		a.ManualContent = ""
	}
}

// AirportsCreate handles POST /admin/airports.
func (d *Deps) AirportsCreate(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	form := parseAirportForm(r)
	errs := validateAirportForm(form)
	if len(errs) > 0 {
		d.rerenderAirportsWithErrors(w, r, form, errs, 0)
		return
	}
	airport := domain.Airport{Enabled: true}
	applyAirportForm(&airport, form)
	if err := d.Airports.Insert(&airport); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if airport.IsManualSource() {
		// Placeholder subUrl above used ID=0 (not yet assigned); regenerate it
		// now that Insert has populated the real ID, then apply the manual content.
		applyAirportForm(&airport, form)
		d.Airports.Update(&airport)
		d.Fetcher.Fetch(airport.ID)
	}
	flashOK(w, buildSaveMessage("已新增机场：", airport))
	http.Redirect(w, r, "/admin/airports", http.StatusFound)
}

// AirportsUpdate handles POST /admin/airports/{id}.
func (d *Deps) AirportsUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	airport, err := d.Airports.FindByID(id)
	if err != nil || airport == nil {
		flashErr(w, "机场不存在 (id="+strconv.FormatInt(id, 10)+")")
		http.Redirect(w, r, "/admin/airports", http.StatusFound)
		return
	}
	r.ParseForm()
	form := parseAirportForm(r)
	errs := validateAirportForm(form)
	if len(errs) > 0 {
		d.rerenderAirportsWithErrors(w, r, form, errs, id)
		return
	}
	applyAirportForm(airport, form)
	if err := d.Airports.Update(airport); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if airport.IsManualSource() {
		d.Fetcher.Fetch(airport.ID)
	}
	flashOK(w, buildSaveMessage("已保存：", *airport))
	http.Redirect(w, r, "/admin/airports", http.StatusFound)
}

func buildSaveMessage(prefix string, a domain.Airport) string {
	base := prefix + a.Name
	if a.IsManualSource() {
		kind := service.DetectContentKind(a.ManualContent)
		return base + "（识别为 " + kind.Label() + "）"
	}
	return base
}

func (d *Deps) rerenderAirportsWithErrors(w http.ResponseWriter, r *http.Request, form airportForm, errs map[string]string, editingID int64) {
	airports, _ := d.Airports.FindAllOrderByID()
	profiles, _ := d.Profiles.FindAllOrderByID()
	render(w, "page:airports", airportsData{
		PageData:  d.pageData(w, r, "机场管理", "airports"),
		Airports:  airports,
		UsedByMap: buildUsedByMap(airports, profiles),
		EditingID: editingID,
		Form:      form,
		Errors:    errs,
	})
}

// AirportsDelete handles POST /admin/airports/{id}/delete.
func (d *Deps) AirportsDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	airport, _ := d.Airports.FindByID(id)
	if airport != nil {
		d.Airports.PurgeFromProfileSelections(id)
		d.Airports.Delete(id)
		flashOK(w, "已删除："+airport.Name)
	}
	http.Redirect(w, r, "/admin/airports", http.StatusFound)
}

// AirportsToggle handles POST /admin/airports/{id}/toggle.
func (d *Deps) AirportsToggle(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	airport, _ := d.Airports.FindByID(id)
	if airport != nil {
		newEnabled := !airport.Enabled
		d.Airports.SetEnabled(id, newEnabled)
		verb := "已停用："
		if newEnabled {
			verb = "已启用："
		}
		flashOK(w, verb+airport.Name)
	}
	http.Redirect(w, r, "/admin/airports", http.StatusFound)
}

// AirportsFetchNow handles POST /admin/airports/{id}/fetch.
func (d *Deps) AirportsFetchNow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(r, "id")
	if !ok {
		http.NotFound(w, r)
		return
	}
	success := d.Fetcher.Fetch(id)
	airport, _ := d.Airports.FindByID(id)
	if airport != nil {
		if success {
			flashOK(w, "已刷新："+airport.Name)
		} else {
			flashErr(w, "刷新失败："+airport.Name+"（原因："+airport.LastFetchError+"）")
		}
	}
	http.Redirect(w, r, "/admin/airports", http.StatusFound)
}
