package store

import (
	"database/sql"
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
		return nil, err
	}

	qb := sq.StatementBuilder.PlaceholderFormat(sq.Question)
	return &Store{db: db, qb: qb, path: path}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate creates the schema and applies incremental migrations.
func migrate(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	// DROP TABLE IF EXISTS fits_cache;
	// DROP TABLE IF EXISTS rejected_files;
	// DROP TABLE IF EXISTS frames;
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS preferences (
			key   TEXT PRIMARY KEY NOT NULL,
			value TEXT NOT NULL
		);
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
			notes            TEXT,
			file_hash        TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_frames_object     ON frames(object);
		CREATE INDEX IF NOT EXISTS idx_frames_filter     ON frames(filter);
		CREATE INDEX IF NOT EXISTS idx_frames_date_obs   ON frames(date_obs);
		CREATE INDEX IF NOT EXISTS idx_frames_obj_filt   ON frames(object, filter);
		CREATE INDEX IF NOT EXISTS idx_frames_rejected   ON frames(rejected);
		CREATE INDEX IF NOT EXISTS idx_frames_frame_type ON frames(frame_type);
		CREATE INDEX IF NOT EXISTS idx_frames_file_hash  ON frames(file_hash);
		CREATE TABLE IF NOT EXISTS projects (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT    NOT NULL,
			description TEXT    NOT NULL DEFAULT '',
			folder      TEXT    NOT NULL UNIQUE,
			created_at  TEXT    NOT NULL
		);
	`)
	if err != nil {
		return err
	}
	// Incremental migrations for existing databases.
	if _, err := db.Exec(`ALTER TABLE frames ADD COLUMN file_hash TEXT`); err != nil {
		// Ignore "duplicate column" errors — the column already exists.
		if !isDuplicateColumnErr(err) {
			return err
		}
	}
	return nil
}

func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) >= 22 && msg[:22] == "duplicate column name:"
}
