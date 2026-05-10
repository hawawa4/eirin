package prefs

import (
	"database/sql"
	"errors"
	"strconv"

	sq "github.com/Masterminds/squirrel"
)

// Preference keys — shared with the frontend via the Prefs struct so both
// sides stay in sync.
const (
	KeyRootFolder        = "root_folder"
	KeyBasicCollapsed    = "basic_collapsed"
	KeyAdvancedCollapsed = "advanced_collapsed"
	KeyStretchEnabled    = "stretch_enabled"
	KeyStretchLevel      = "stretch_level"
	KeyColumnConfig        = "column_config"
	KeyLibraryColumnConfig = "library_column_config"
)

// Prefs is the typed snapshot of all user preferences, serialised to/from the
// database as plain strings.
type Prefs struct {
	RootFolder           string `json:"rootFolder"`
	BasicCollapsed       bool   `json:"basicCollapsed"`
	AdvancedCollapsed    bool   `json:"advancedCollapsed"`
	StretchEnabled       bool   `json:"stretchEnabled"`
	StretchLevel         int    `json:"stretchLevel"`
	ColumnConfig         string `json:"columnConfig"`
	LibraryColumnConfig  string `json:"libraryColumnConfig"`
}

// DefaultPrefs returns the out-of-the-box preference values.
func DefaultPrefs() Prefs {
	return Prefs{
		BasicCollapsed:    false,
		AdvancedCollapsed: true,
		StretchEnabled:    true,
		StretchLevel:      2,
		ColumnConfig:      "",
		LibraryColumnConfig: "",
	}
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
	if v, ok := s.getString(KeyLibraryColumnConfig); ok {
		p.LibraryColumnConfig = v
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
