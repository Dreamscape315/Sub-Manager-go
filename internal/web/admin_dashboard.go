package web

import (
	"net/http"
	"time"

	"github.com/akatsukisky/sub-manager-go/internal/domain"
	"github.com/akatsukisky/sub-manager-go/internal/service"
)

type dashboardData struct {
	PageData
	Airports         []domain.Airport
	Profiles         []domain.Profile
	EnabledAirports  int
	TotalAirports    int
	OKAirports       int
	EnabledProfiles  int
	TotalProfiles    int
	OKProfiles       int
	TotalNodes       int
	ProfileNodeCount map[int64]int
	LastRefresh      time.Time
}

// Root redirects "/" to the dashboard.
func (d *Deps) Root(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin", http.StatusFound)
}

// Dashboard renders the overview page: aggregate health/counts across every
// airport and profile, so the admin can see system status at a glance.
func (d *Deps) Dashboard(w http.ResponseWriter, r *http.Request) {
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

	var enabledAirports, okAirports int
	for _, a := range airports {
		if a.Enabled {
			enabledAirports++
			if a.LastFetchOK {
				okAirports++
			}
		}
	}
	var enabledProfiles, okProfiles int
	for _, p := range profiles {
		if p.Enabled {
			enabledProfiles++
			if p.HasCachedContent() {
				okProfiles++
			}
		}
	}

	var lastRefresh time.Time
	for _, a := range airports {
		if a.LastFetchedAt.After(lastRefresh) {
			lastRefresh = a.LastFetchedAt
		}
	}
	for _, p := range profiles {
		if p.LastGeneratedAt.After(lastRefresh) {
			lastRefresh = p.LastGeneratedAt
		}
	}

	nodeCount := make(map[int64]int, len(profiles))
	var totalNodes int
	for _, p := range profiles {
		n := service.CountClashNodes(p.CachedContent)
		nodeCount[p.ID] = n
		if p.Enabled {
			totalNodes += n
		}
	}

	render(w, "page:dashboard", dashboardData{
		PageData:         d.pageData(w, r, "概览", "dashboard"),
		Airports:         airports,
		Profiles:         profiles,
		EnabledAirports:  enabledAirports,
		TotalAirports:    len(airports),
		OKAirports:       okAirports,
		EnabledProfiles:  enabledProfiles,
		TotalProfiles:    len(profiles),
		OKProfiles:       okProfiles,
		TotalNodes:       totalNodes,
		ProfileNodeCount: nodeCount,
		LastRefresh:      lastRefresh,
	})
}

func (d *Deps) pageData(w http.ResponseWriter, r *http.Request, title, nav string) PageData {
	return PageData{
		Title:         title,
		Nav:           nav,
		PublicBaseURL: d.publicBaseURL(),
		FlashOK:       popFlashOK(w, r),
		FlashErr:      popFlashErr(w, r),
	}
}
