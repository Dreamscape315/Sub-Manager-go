package domain

import "database/sql"

type SettingsRepo struct{ db *sql.DB }

func NewSettingsRepo(db *sql.DB) *SettingsRepo { return &SettingsRepo{db: db} }

const settingsColumns = `id, refresh_interval_seconds, subconverter_timeout_seconds,
	airport_fetch_timeout_seconds, subconverter_base_url, internal_base_url,
	public_base_url, internal_secret`

// Get reads the singleton settings row, inserting defaults on first run.
func (r *SettingsRepo) Get() (AppSettings, error) {
	row := r.db.QueryRow(`SELECT `+settingsColumns+` FROM app_settings WHERE id = ?`, SettingsSingletonID)
	var s AppSettings
	var publicBaseURL, internalSecret sql.NullString
	err := row.Scan(&s.ID, &s.RefreshIntervalSeconds, &s.SubconverterTimeoutSeconds,
		&s.AirportFetchTimeoutSeconds, &s.SubconverterBaseURL, &s.InternalBaseURL,
		&publicBaseURL, &internalSecret)
	if err == sql.ErrNoRows {
		defaults := DefaultSettings()
		if err := r.Save(defaults); err != nil {
			return AppSettings{}, err
		}
		return defaults, nil
	}
	if err != nil {
		return AppSettings{}, err
	}
	s.PublicBaseURL = publicBaseURL.String
	s.InternalSecret = internalSecret.String
	return s, nil
}

// Save persists the settings row, forcing id = 1 regardless of input.
func (r *SettingsRepo) Save(s AppSettings) error {
	_, err := r.db.Exec(`INSERT INTO app_settings (id, refresh_interval_seconds,
			subconverter_timeout_seconds, airport_fetch_timeout_seconds,
			subconverter_base_url, internal_base_url, public_base_url, internal_secret)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			refresh_interval_seconds = excluded.refresh_interval_seconds,
			subconverter_timeout_seconds = excluded.subconverter_timeout_seconds,
			airport_fetch_timeout_seconds = excluded.airport_fetch_timeout_seconds,
			subconverter_base_url = excluded.subconverter_base_url,
			internal_base_url = excluded.internal_base_url,
			public_base_url = excluded.public_base_url,
			internal_secret = excluded.internal_secret`,
		SettingsSingletonID, s.RefreshIntervalSeconds, s.SubconverterTimeoutSeconds,
		s.AirportFetchTimeoutSeconds, s.SubconverterBaseURL, s.InternalBaseURL,
		nullable(s.PublicBaseURL), nullable(s.InternalSecret))
	return err
}
