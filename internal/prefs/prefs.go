package prefs

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"

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
)

// Prefs is the typed snapshot of all user preferences, serialised to/from the
// database as plain strings.
type Prefs struct {
	RootFolder        string `json:"rootFolder"`
	BasicCollapsed    bool   `json:"basicCollapsed"`
	AdvancedCollapsed bool   `json:"advancedCollapsed"`
	StretchEnabled    bool   `json:"stretchEnabled"`
	StretchLevel      int    `json:"stretchLevel"`
}

// DefaultPrefs returns the out-of-the-box preference values.
func DefaultPrefs() Prefs {
	return Prefs{
		BasicCollapsed:    false,
		AdvancedCollapsed: true,
		StretchEnabled:    true,
		StretchLevel:      2,
	}
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
		db.Close()
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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS preferences (
		key   TEXT PRIMARY KEY NOT NULL,
		value TEXT NOT NULL
	)`)
	return err
}
