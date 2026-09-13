package domain

import (
	"database/sql"
	"time"
)

type AirportRepo struct{ db *sql.DB }

func NewAirportRepo(db *sql.DB) *AirportRepo { return &AirportRepo{db: db} }

func scanAirport(row interface{ Scan(...any) error }) (Airport, error) {
	var a Airport
	var manualContent, cachedRaw, cachedUserinfo, lastFetchedAt, lastFetchError sql.NullString
	var enabled, lastFetchOK int
	err := row.Scan(&a.ID, &a.Name, &a.SubURL, &a.SourceType, &manualContent,
		&enabled, &cachedRaw, &cachedUserinfo, &lastFetchedAt, &lastFetchOK, &lastFetchError)
	if err != nil {
		return Airport{}, err
	}
	a.ManualContent = manualContent.String
	a.CachedRawContent = cachedRaw.String
	a.CachedUserinfo = cachedUserinfo.String
	a.LastFetchError = lastFetchError.String
	a.Enabled = enabled != 0
	a.LastFetchOK = lastFetchOK != 0
	if lastFetchedAt.Valid && lastFetchedAt.String != "" {
		if t, perr := time.Parse(time.RFC3339Nano, lastFetchedAt.String); perr == nil {
			a.LastFetchedAt = t
		}
	}
	return a, nil
}

const airportColumns = `id, name, sub_url, source_type, manual_content,
	enabled, cached_raw_content, cached_userinfo, last_fetched_at, last_fetch_ok, last_fetch_error`

func (r *AirportRepo) FindAllOrderByID() ([]Airport, error) {
	rows, err := r.db.Query(`SELECT ` + airportColumns + ` FROM airport ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Airport
	for rows.Next() {
		a, err := scanAirport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AirportRepo) FindEnabledOrderByID() ([]Airport, error) {
	rows, err := r.db.Query(`SELECT ` + airportColumns + ` FROM airport WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Airport
	for rows.Next() {
		a, err := scanAirport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *AirportRepo) FindByID(id int64) (*Airport, error) {
	row := r.db.QueryRow(`SELECT `+airportColumns+` FROM airport WHERE id = ?`, id)
	a, err := scanAirport(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AirportRepo) Insert(a *Airport) error {
	res, err := r.db.Exec(`INSERT INTO airport (name, sub_url, source_type, manual_content, enabled)
		VALUES (?, ?, ?, ?, ?)`,
		a.Name, a.SubURL, a.SourceType, nullable(a.ManualContent), boolToInt(a.Enabled))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

func (r *AirportRepo) Update(a *Airport) error {
	_, err := r.db.Exec(`UPDATE airport SET name = ?, sub_url = ?, source_type = ?, manual_content = ?
		WHERE id = ?`,
		a.Name, a.SubURL, a.SourceType, nullable(a.ManualContent), a.ID)
	return err
}

func (r *AirportRepo) SetEnabled(id int64, enabled bool) error {
	_, err := r.db.Exec(`UPDATE airport SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	return err
}

func (r *AirportRepo) Delete(id int64) error {
	_, err := r.db.Exec(`DELETE FROM airport WHERE id = ?`, id)
	return err
}

// MarkFetchSuccess atomically writes the fetched content, Subscription-Userinfo
// header and timestamp, clearing any previous error.
func (r *AirportRepo) MarkFetchSuccess(id int64, content, userinfo string, at time.Time) error {
	_, err := r.db.Exec(`UPDATE airport SET cached_raw_content = ?, cached_userinfo = ?,
		last_fetched_at = ?, last_fetch_ok = 1, last_fetch_error = NULL WHERE id = ?`,
		content, nullable(userinfo), at.UTC().Format(time.RFC3339Nano), id)
	return err
}

// MarkFetchFailure records a failed fetch attempt without touching the cached content.
func (r *AirportRepo) MarkFetchFailure(id int64, at time.Time, errMsg string) error {
	_, err := r.db.Exec(`UPDATE airport SET last_fetched_at = ?, last_fetch_ok = 0,
		last_fetch_error = ? WHERE id = ?`,
		at.UTC().Format(time.RFC3339Nano), errMsg, id)
	return err
}

// PurgeFromProfileSelections removes airportID from every profile's selected set,
// avoiding orphaned rows in profile_airports after an airport is deleted.
func (r *AirportRepo) PurgeFromProfileSelections(airportID int64) error {
	_, err := r.db.Exec(`DELETE FROM profile_airports WHERE airport_id = ?`, airportID)
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
