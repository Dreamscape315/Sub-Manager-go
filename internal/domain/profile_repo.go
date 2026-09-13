package domain

import (
	"database/sql"
	"time"
)

type ProfileRepo struct{ db *sql.DB }

func NewProfileRepo(db *sql.DB) *ProfileRepo { return &ProfileRepo{db: db} }

const profileColumns = `id, slug, target_format, external_config, enabled,
	cached_content, cached_content_type, last_generated_at`

func scanProfile(row interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	var externalConfig, cachedContent, cachedContentType, lastGeneratedAt sql.NullString
	var enabled int
	err := row.Scan(&p.ID, &p.Slug, &p.TargetFormat, &externalConfig, &enabled,
		&cachedContent, &cachedContentType, &lastGeneratedAt)
	if err != nil {
		return Profile{}, err
	}
	p.ExternalConfig = externalConfig.String
	p.CachedContent = cachedContent.String
	p.CachedContentType = cachedContentType.String
	p.Enabled = enabled != 0
	if lastGeneratedAt.Valid && lastGeneratedAt.String != "" {
		if t, perr := time.Parse(time.RFC3339Nano, lastGeneratedAt.String); perr == nil {
			p.LastGeneratedAt = t
		}
	}
	return p, nil
}

func (r *ProfileRepo) loadSelectedAirportIDs(profileID int64) ([]int64, error) {
	rows, err := r.db.Query(`SELECT airport_id FROM profile_airports WHERE profile_id = ? ORDER BY airport_id ASC`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *ProfileRepo) hydrate(profiles []Profile) error {
	for i := range profiles {
		ids, err := r.loadSelectedAirportIDs(profiles[i].ID)
		if err != nil {
			return err
		}
		profiles[i].SelectedAirportIDs = ids
	}
	return nil
}

func (r *ProfileRepo) FindAllOrderByID() ([]Profile, error) {
	rows, err := r.db.Query(`SELECT ` + profileColumns + ` FROM profile ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.hydrate(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ProfileRepo) FindEnabledOrderByID() ([]Profile, error) {
	rows, err := r.db.Query(`SELECT ` + profileColumns + ` FROM profile WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.hydrate(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *ProfileRepo) FindByID(id int64) (*Profile, error) {
	row := r.db.QueryRow(`SELECT `+profileColumns+` FROM profile WHERE id = ?`, id)
	p, err := scanProfile(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids, err := r.loadSelectedAirportIDs(p.ID)
	if err != nil {
		return nil, err
	}
	p.SelectedAirportIDs = ids
	return &p, nil
}

func (r *ProfileRepo) FindBySlug(slug string) (*Profile, error) {
	row := r.db.QueryRow(`SELECT `+profileColumns+` FROM profile WHERE slug = ?`, slug)
	p, err := scanProfile(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids, err := r.loadSelectedAirportIDs(p.ID)
	if err != nil {
		return nil, err
	}
	p.SelectedAirportIDs = ids
	return &p, nil
}

func (r *ProfileRepo) ExistsBySlugExcludingID(slug string, excludeID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM profile WHERE slug = ? AND id != ?`, slug, excludeID).Scan(&count)
	return count > 0, err
}

func (r *ProfileRepo) Insert(p *Profile) error {
	res, err := r.db.Exec(`INSERT INTO profile (slug, target_format, external_config, enabled)
		VALUES (?, ?, ?, ?)`,
		p.Slug, p.TargetFormat, nullable(p.ExternalConfig), boolToInt(p.Enabled))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return r.saveSelectedAirportIDs(p.ID, p.SelectedAirportIDs)
}

func (r *ProfileRepo) Update(p *Profile) error {
	_, err := r.db.Exec(`UPDATE profile SET slug = ?, target_format = ?, external_config = ? WHERE id = ?`,
		p.Slug, p.TargetFormat, nullable(p.ExternalConfig), p.ID)
	if err != nil {
		return err
	}
	return r.saveSelectedAirportIDs(p.ID, p.SelectedAirportIDs)
}

func (r *ProfileRepo) saveSelectedAirportIDs(profileID int64, ids []int64) error {
	if _, err := r.db.Exec(`DELETE FROM profile_airports WHERE profile_id = ?`, profileID); err != nil {
		return err
	}
	for _, id := range ids {
		if _, err := r.db.Exec(`INSERT OR IGNORE INTO profile_airports (profile_id, airport_id) VALUES (?, ?)`,
			profileID, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProfileRepo) SetEnabled(id int64, enabled bool) error {
	_, err := r.db.Exec(`UPDATE profile SET enabled = ? WHERE id = ?`, boolToInt(enabled), id)
	return err
}

func (r *ProfileRepo) Delete(id int64) error {
	if _, err := r.db.Exec(`DELETE FROM profile_airports WHERE profile_id = ?`, id); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM profile WHERE id = ?`, id)
	return err
}

// UpdateCachedContent atomically writes the synthesized subscription text.
func (r *ProfileRepo) UpdateCachedContent(id int64, content, contentType string, at time.Time) error {
	_, err := r.db.Exec(`UPDATE profile SET cached_content = ?, cached_content_type = ?,
		last_generated_at = ? WHERE id = ?`,
		content, nullable(contentType), at.UTC().Format(time.RFC3339Nano), id)
	return err
}
