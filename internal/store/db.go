package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	sq "github.com/Masterminds/squirrel"
	_ "modernc.org/sqlite"
)

// EnvDBPath is the environment variable that overrides the default database path.
// Set it to a custom file path to redirect the database away from the platform
// config directory — used in tests and for user-configurable storage locations.
const EnvDBPath = "EIRIN_DB_PATH"

// Store is the SQLite-backed preference and frame repository.
type Store struct {
	db   *sql.DB
	qb   sq.StatementBuilderType
	path string // filesystem path of the database file
}

// DBPath returns the filesystem path of the SQLite database file.
func (s *Store) DBPath() string { return s.path }

// NewStore opens (or creates) the SQLite database. If the EIRIN_DB_PATH
// environment variable is set, it is used as the path; otherwise the platform
// config directory ($XDG_CONFIG_HOME/eirin/prefs.db on Linux, etc.) is used.
func NewStore() (*Store, error) {
	if path := os.Getenv(EnvDBPath); path != "" {
		return NewStoreAt(path)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return NewStoreAt(filepath.Join(dir, "eirin", "prefs.db"))
}

// NewStoreAt opens (or creates) a SQLite database at an explicit path.
// The parent directory is created automatically when it does not exist.
func NewStoreAt(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: single writer

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	qb := sq.StatementBuilder.PlaceholderFormat(sq.Question)
	return &Store{db: db, qb: qb, path: path}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate runs all pending migrations against db.
// Migrations are numbered sequentially; each is applied exactly once and the
// version is recorded in the schema_migrations table.
func migrate(db *sql.DB) error {
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return err
	}
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version   INTEGER PRIMARY KEY NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (datetime('now'))
		)
	`); err != nil {
		return err
	}

	for _, m := range migrations {
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version=?`, m.version).Scan(&count); err != nil {
			return fmt.Errorf("migration %d: check: %w", m.version, err)
		}
		if count > 0 {
			continue
		}
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("migration %d: %w", m.version, err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			return fmt.Errorf("migration %d: record: %w", m.version, err)
		}
	}
	return nil
}

type migration struct {
	version int
	sql     string
}

// migrations is the ordered list of schema changes. Never edit an existing
// entry — add a new one at the end for every schema change.
var migrations = []migration{
	{1, `
		CREATE TABLE IF NOT EXISTS preferences (
			key   TEXT PRIMARY KEY NOT NULL,
			value TEXT NOT NULL
		)
	`},
	{2, `
		CREATE TABLE IF NOT EXISTS frames (
			nas_path    TEXT PRIMARY KEY NOT NULL,
			file_size   INTEGER,
			last_seen   INTEGER,
			cached_at   INTEGER NOT NULL DEFAULT 0,

			object      TEXT NOT NULL DEFAULT '',
			filter      TEXT NOT NULL DEFAULT '',
			exptime     REAL NOT NULL DEFAULT 0,
			gain        REAL NOT NULL DEFAULT 0,
			ccd_temp    REAL NOT NULL DEFAULT 0,
			date_obs    TEXT NOT NULL DEFAULT '',
			telescope   TEXT NOT NULL DEFAULT '',
			instrument  TEXT NOT NULL DEFAULT '',

			frame_type  TEXT NOT NULL DEFAULT 'stacked',

			ra          REAL,
			dec         REAL,
			pixel_scale REAL,
			rotation    REAL,
			wcs_solved  INTEGER NOT NULL DEFAULT 0,

			fwhm        REAL,
			fwhm_unit   TEXT,
			roundness   REAL,
			background  REAL,
			noise       REAL,
			snr         REAL,
			star_count  INTEGER,
			quality_analyzed INTEGER NOT NULL DEFAULT 0,

			approved         INTEGER,
			rejected         INTEGER NOT NULL DEFAULT 0,
			rejection_reason TEXT,
			tags             TEXT,
			notes            TEXT
		)
	`},
	{3, `CREATE INDEX IF NOT EXISTS idx_frames_object     ON frames(object)`},
	{4, `CREATE INDEX IF NOT EXISTS idx_frames_filter     ON frames(filter)`},
	{5, `CREATE INDEX IF NOT EXISTS idx_frames_date_obs   ON frames(date_obs)`},
	{6, `CREATE INDEX IF NOT EXISTS idx_frames_obj_filt   ON frames(object, filter)`},
	{7, `CREATE INDEX IF NOT EXISTS idx_frames_rejected   ON frames(rejected)`},
	{8, `CREATE INDEX IF NOT EXISTS idx_frames_frame_type ON frames(frame_type)`},
	{9, `
		CREATE TABLE IF NOT EXISTS projects (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL,
			description TEXT    NOT NULL DEFAULT '',
			folder      TEXT    NOT NULL UNIQUE,
			created_at  TEXT    NOT NULL
		)
	`},
	// migration 10: add file_hash for content-based duplicate detection
	{10, `ALTER TABLE frames ADD COLUMN file_hash TEXT`},
	{11, `CREATE INDEX IF NOT EXISTS idx_frames_file_hash ON frames(file_hash)`},
}
