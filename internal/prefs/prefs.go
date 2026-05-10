package prefs

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"
	_ "modernc.org/sqlite"
)

// Preference keys. Shared with the frontend via the exported Prefs struct and
// the key constants so both sides stay in sync.
const (
	KeyRootFolder        = "root_folder"
	KeyBasicCollapsed    = "basic_collapsed"
	KeyAdvancedCollapsed = "advanced_collapsed"
	KeyStretchEnabled    = "stretch_enabled"
	KeyStretchLevel      = "stretch_level"
	KeyColumnConfig      = "column_config"
)

// Prefs is the typed snapshot of all user preferences, serialised to/from the
// database as plain strings.
type Prefs struct {
	RootFolder        string `json:"rootFolder"`
	BasicCollapsed    bool   `json:"basicCollapsed"`
	AdvancedCollapsed bool   `json:"advancedCollapsed"`
	StretchEnabled    bool   `json:"stretchEnabled"`
	StretchLevel      int    `json:"stretchLevel"`
	ColumnConfig      string `json:"columnConfig"`
}

// DefaultPrefs returns the out-of-the-box preference values.
func DefaultPrefs() Prefs {
	return Prefs{
		BasicCollapsed:    false,
		AdvancedCollapsed: true,
		StretchEnabled:    true,
		StretchLevel:      2,
		ColumnConfig:      "",
	}
}

// CachedFITSHeader holds the subset of FITS header fields stored in the local
// cache table so directory listings can show metadata without re-reading files.
type CachedFITSHeader struct {
	Object     string
	Filter     string
	ExpTime    float64
	DateObs    string
	Gain       float64
	CCDTemp    float64
	Telescope  string
	Instrument string
}

// Store is the SQLite-backed preference repository.
type Store struct {
	db *sql.DB
	qb sq.StatementBuilderType
}

// NewStore opens (or creates) the SQLite database in the platform config dir
// ($XDG_CONFIG_HOME/eirin/prefs.db on Linux, ~/Library/… on macOS, etc.).
func NewStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	dir = filepath.Join(dir, "eirin")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dir, "prefs.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: single writer

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	qb := sq.StatementBuilder.PlaceholderFormat(sq.Question)
	return &Store{db: db, qb: qb}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// Load reads all preferences from the database, filling in defaults for any
// missing keys.
func (s *Store) Load() Prefs {
	p := DefaultPrefs()
	if v, ok := s.getString(KeyRootFolder); ok {
		p.RootFolder = v
	}
	if v, ok := s.getString(KeyBasicCollapsed); ok {
		p.BasicCollapsed = v == "true"
	}
	if v, ok := s.getString(KeyAdvancedCollapsed); ok {
		p.AdvancedCollapsed = v == "true"
	}
	if v, ok := s.getString(KeyStretchEnabled); ok {
		p.StretchEnabled = v == "true"
	}
	if v, ok := s.getString(KeyStretchLevel); ok {
		if n, err := strconv.Atoi(v); err == nil {
			p.StretchLevel = n
		}
	}
	if v, ok := s.getString(KeyColumnConfig); ok {
		p.ColumnConfig = v
	}
	return p
}

// Set persists a single preference by key. Value is always stored as a string.
func (s *Store) Set(key, value string) error {
	query, args, err := s.qb.
		Insert("preferences").
		Columns("key", "value").
		Values(key, value).
		Suffix("ON CONFLICT(key) DO UPDATE SET value = excluded.value").
		ToSql()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(query, args...)
	return err
}

// UpsertFITSCache stores or updates the cached FITS header for a single file.
func (s *Store) UpsertFITSCache(path string, h CachedFITSHeader) error {
	query, args, err := s.qb.
		Insert("fits_cache").
		Columns("path", "object", "filter", "exp_time", "date_obs",
			"gain", "ccd_temp", "telescope", "instrument", "cached_at").
		Values(path, h.Object, h.Filter, h.ExpTime, h.DateObs,
			h.Gain, h.CCDTemp, h.Telescope, h.Instrument, time.Now().Unix()).
		Suffix(`ON CONFLICT(path) DO UPDATE SET
			object=excluded.object, filter=excluded.filter,
			exp_time=excluded.exp_time, date_obs=excluded.date_obs,
			gain=excluded.gain, ccd_temp=excluded.ccd_temp,
			telescope=excluded.telescope, instrument=excluded.instrument,
			cached_at=excluded.cached_at`).
		ToSql()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(query, args...)
	return err
}

// GetFITSCache retrieves cached FITS headers for the given file paths.
// Only paths that have a cached entry are included in the returned map.
func (s *Store) GetFITSCache(paths []string) (map[string]CachedFITSHeader, error) {
	result := make(map[string]CachedFITSHeader, len(paths))
	if len(paths) == 0 {
		return result, nil
	}
	query, args, err := s.qb.
		Select("path", "object", "filter", "exp_time", "date_obs",
			"gain", "ccd_temp", "telescope", "instrument").
		From("fits_cache").
		Where(sq.Eq{"path": paths}).
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		var h CachedFITSHeader
		if err := rows.Scan(&p, &h.Object, &h.Filter, &h.ExpTime, &h.DateObs,
			&h.Gain, &h.CCDTemp, &h.Telescope, &h.Instrument); err != nil {
			return nil, err
		}
		result[p] = h
	}
	return result, rows.Err()
}

// getString retrieves a single value by key. Returns ("", false) when absent.
func (s *Store) getString(key string) (string, bool) {
	query, args, err := s.qb.
		Select("value").
		From("preferences").
		Where(sq.Eq{"key": key}).
		ToSql()
	if err != nil {
		return "", false
	}
	var val string
	if err := s.db.QueryRow(query, args...).Scan(&val); errors.Is(err, sql.ErrNoRows) || err != nil {
		return "", false
	}
	return val, true
}

// migrate creates the schema if it does not already exist.
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS preferences (
			key   TEXT PRIMARY KEY NOT NULL,
			value TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS fits_cache (
			path       TEXT    PRIMARY KEY NOT NULL,
			object     TEXT    NOT NULL DEFAULT '',
			filter     TEXT    NOT NULL DEFAULT '',
			exp_time   REAL    NOT NULL DEFAULT 0,
			date_obs   TEXT    NOT NULL DEFAULT '',
			gain       REAL    NOT NULL DEFAULT 0,
			ccd_temp   REAL    NOT NULL DEFAULT 0,
			telescope  TEXT    NOT NULL DEFAULT '',
			instrument TEXT    NOT NULL DEFAULT '',
			cached_at  INTEGER NOT NULL DEFAULT 0
		)
	`)
	return err
}
