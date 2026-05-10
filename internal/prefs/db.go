package prefs

import (
	"database/sql"
	"os"
	"path/filepath"

	sq "github.com/Masterminds/squirrel"
	_ "modernc.org/sqlite"
)

// Store is the SQLite-backed preference and frame repository.
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

// migrate creates the schema. Old tables are dropped on startup since data
// migration is not required during development.
func migrate(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	_, err := db.Exec(`
		DROP TABLE IF EXISTS fits_cache;
		DROP TABLE IF EXISTS rejected_files;
		DROP TABLE IF EXISTS frames;
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
			notes            TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_frames_object     ON frames(object);
		CREATE INDEX IF NOT EXISTS idx_frames_filter     ON frames(filter);
		CREATE INDEX IF NOT EXISTS idx_frames_date_obs   ON frames(date_obs);
		CREATE INDEX IF NOT EXISTS idx_frames_obj_filt   ON frames(object, filter);
		CREATE INDEX IF NOT EXISTS idx_frames_rejected   ON frames(rejected);
		CREATE INDEX IF NOT EXISTS idx_frames_frame_type ON frames(frame_type);
	`)
	return err
}
