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

// Frame holds all data stored about a single file in the frames table.
// FITS header fields are populated by the indexer; WCS and quality fields are
// filled later by plate-solving and Siril analysis; app metadata is set by the user.
// CachedAt == 0 means FITS data has not yet been indexed for this path.
type Frame struct {
	NasPath  string
	FileSize int64
	LastSeen int64 // unix timestamp
	CachedAt int64 // unix timestamp; 0 = not yet indexed

	// FITS header
	Object     string
	Filter     string
	ExpTime    float64
	Gain       float64
	CCDTemp    float64
	DateObs    string
	Telescope  string
	Instrument string

	// Plate solve results (WCS)
	RA         *float64
	Dec        *float64
	PixelScale *float64
	Rotation   *float64
	WCSSolved  bool

	// Quality metrics from Siril
	FWHM            *float64
	FWHMUnit        string
	Roundness       *float64
	Background      *float64
	Noise           *float64
	SNR             *float64
	StarCount       *int64
	QualityAnalyzed bool

	// App metadata
	Approved        *bool
	Rejected        bool
	RejectionReason string
	Tags            string // JSON array
	Notes           string
}

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

// UpsertFrame stores or updates the FITS header fields for a single frame.
// Only the header-derived columns are written; user metadata (rejected, tags, etc.)
// and analysis results (wcs, quality) are preserved if the row already exists.
func (s *Store) UpsertFrame(path string, f Frame) error {
	query, args, err := s.qb.
		Insert("frames").
		Columns("nas_path", "file_size", "last_seen", "cached_at",
			"object", "filter", "exptime", "gain", "ccd_temp", "date_obs",
			"telescope", "instrument").
		Values(path, f.FileSize, f.LastSeen, time.Now().Unix(),
			f.Object, f.Filter, f.ExpTime, f.Gain, f.CCDTemp, f.DateObs,
			f.Telescope, f.Instrument).
		Suffix(`ON CONFLICT(nas_path) DO UPDATE SET
			file_size=excluded.file_size, last_seen=excluded.last_seen,
			cached_at=excluded.cached_at,
			object=excluded.object, filter=excluded.filter, exptime=excluded.exptime,
			gain=excluded.gain, ccd_temp=excluded.ccd_temp, date_obs=excluded.date_obs,
			telescope=excluded.telescope, instrument=excluded.instrument`).
		ToSql()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(query, args...)
	return err
}

// BatchUpsertFrames inserts or updates FITS header data for many frames in a
// single transaction. Only header-derived columns are written; user metadata
// and analysis results are preserved on conflict.
func (s *Store) BatchUpsertFrames(entries map[string]Frame) error {
	if len(entries) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(`
		INSERT INTO frames
			(nas_path, file_size, last_seen, cached_at,
			 object, filter, exptime, gain, ccd_temp, date_obs, telescope, instrument)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(nas_path) DO UPDATE SET
			file_size=excluded.file_size, last_seen=excluded.last_seen,
			cached_at=excluded.cached_at,
			object=excluded.object, filter=excluded.filter, exptime=excluded.exptime,
			gain=excluded.gain, ccd_temp=excluded.ccd_temp, date_obs=excluded.date_obs,
			telescope=excluded.telescope, instrument=excluded.instrument
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for path, f := range entries {
		if _, err := stmt.Exec(path, f.FileSize, f.LastSeen, now,
			f.Object, f.Filter, f.ExpTime, f.Gain, f.CCDTemp, f.DateObs,
			f.Telescope, f.Instrument); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetFrames retrieves frames for the given file paths. Only paths present in
// the frames table are included in the result. Queries are chunked to stay
// within SQLite's variable limit.
func (s *Store) GetFrames(paths []string) (map[string]Frame, error) {
	result := make(map[string]Frame, len(paths))
	if len(paths) == 0 {
		return result, nil
	}
	const chunkSize = 900
	for i := 0; i < len(paths); i += chunkSize {
		end := i + chunkSize
		if end > len(paths) {
			end = len(paths)
		}
		if err := s.getFramesChunk(paths[i:end], result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (s *Store) getFramesChunk(paths []string, out map[string]Frame) error {
	query, args, err := s.qb.
		Select(
			"nas_path", "file_size", "last_seen", "cached_at",
			"object", "filter", "exptime", "gain", "ccd_temp", "date_obs", "telescope", "instrument",
			"ra", "dec", "pixel_scale", "rotation", "wcs_solved",
			"fwhm", "fwhm_unit", "roundness", "background", "noise", "snr", "star_count", "quality_analyzed",
			"approved", "rejected", "rejection_reason", "tags", "notes",
		).
		From("frames").
		Where(sq.Eq{"nas_path": paths}).
		ToSql()
	if err != nil {
		return err
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		path, f, err := scanFrame(rows)
		if err != nil {
			return err
		}
		out[path] = f
	}
	return rows.Err()
}

func scanFrame(rows *sql.Rows) (string, Frame, error) {
	var (
		path     string
		fileSize sql.NullInt64
		lastSeen sql.NullInt64
		cachedAt int64

		object, filter, dateObs, telescope, instrument string
		exptime, gain, ccdTemp                         float64

		ra, dec, pixScale, rotation            sql.NullFloat64
		wcsSolved                              int64
		fwhm, roundness, background, noise, snr sql.NullFloat64
		fwhmUnit                               sql.NullString
		starCount                              sql.NullInt64
		qualityAnalyzed                        int64

		approved        sql.NullInt64
		rejected        int64
		rejectionReason sql.NullString
		tags, notes     sql.NullString
	)

	err := rows.Scan(
		&path, &fileSize, &lastSeen, &cachedAt,
		&object, &filter, &exptime, &gain, &ccdTemp, &dateObs, &telescope, &instrument,
		&ra, &dec, &pixScale, &rotation, &wcsSolved,
		&fwhm, &fwhmUnit, &roundness, &background, &noise, &snr, &starCount, &qualityAnalyzed,
		&approved, &rejected, &rejectionReason, &tags, &notes,
	)
	if err != nil {
		return "", Frame{}, err
	}

	f := Frame{
		NasPath:         path,
		CachedAt:        cachedAt,
		Object:          object,
		Filter:          filter,
		ExpTime:         exptime,
		Gain:            gain,
		CCDTemp:         ccdTemp,
		DateObs:         dateObs,
		Telescope:       telescope,
		Instrument:      instrument,
		WCSSolved:       wcsSolved != 0,
		QualityAnalyzed: qualityAnalyzed != 0,
		Rejected:        rejected != 0,
	}
	if fileSize.Valid {
		f.FileSize = fileSize.Int64
	}
	if lastSeen.Valid {
		f.LastSeen = lastSeen.Int64
	}
	if ra.Valid {
		v := ra.Float64
		f.RA = &v
	}
	if dec.Valid {
		v := dec.Float64
		f.Dec = &v
	}
	if pixScale.Valid {
		v := pixScale.Float64
		f.PixelScale = &v
	}
	if rotation.Valid {
		v := rotation.Float64
		f.Rotation = &v
	}
	if fwhm.Valid {
		v := fwhm.Float64
		f.FWHM = &v
	}
	if fwhmUnit.Valid {
		f.FWHMUnit = fwhmUnit.String
	}
	if roundness.Valid {
		v := roundness.Float64
		f.Roundness = &v
	}
	if background.Valid {
		v := background.Float64
		f.Background = &v
	}
	if noise.Valid {
		v := noise.Float64
		f.Noise = &v
	}
	if snr.Valid {
		v := snr.Float64
		f.SNR = &v
	}
	if starCount.Valid {
		f.StarCount = &starCount.Int64
	}
	if approved.Valid {
		v := approved.Int64 != 0
		f.Approved = &v
	}
	if rejectionReason.Valid {
		f.RejectionReason = rejectionReason.String
	}
	if tags.Valid {
		f.Tags = tags.String
	}
	if notes.Valid {
		f.Notes = notes.String
	}

	return path, f, nil
}

// RejectFrame marks a file as rejected with an optional reason. If the file
// does not yet have a frame record, one is created with only the rejection data.
func (s *Store) RejectFrame(path, reason string) error {
	_, err := s.db.Exec(`
		INSERT INTO frames (nas_path, rejected, rejection_reason, cached_at)
		VALUES (?, 1, ?, 0)
		ON CONFLICT(nas_path) DO UPDATE SET
			rejected=1, rejection_reason=excluded.rejection_reason
	`, path, reason)
	return err
}

// UnrejectFrame clears the rejection status of a frame.
func (s *Store) UnrejectFrame(path string) error {
	_, err := s.db.Exec(
		`UPDATE frames SET rejected=0, rejection_reason=NULL WHERE nas_path=?`,
		path,
	)
	return err
}

// GetAllRejectedUnder returns all rejected nas_paths that start with rootPath.
func (s *Store) GetAllRejectedUnder(rootPath string) ([]string, error) {
	prefix := rootPath
	if len(prefix) > 0 && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	query, args, err := s.qb.
		Select("nas_path").
		From("frames").
		Where(sq.And{
			sq.Like{"nas_path": prefix + "%"},
			sq.Eq{"rejected": 1},
		}).
		OrderBy("nas_path").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// DeleteFrame removes a frame record entirely from the database.
func (s *Store) DeleteFrame(path string) error {
	query, args, err := s.qb.
		Delete("frames").
		Where(sq.Eq{"nas_path": path}).
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

// migrate creates the schema. The old fits_cache and rejected_files tables are
// dropped on startup since data migration is not required during development.
func migrate(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	_, err := db.Exec(`
		DROP TABLE IF EXISTS fits_cache;
		DROP TABLE IF EXISTS rejected_files;
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
		CREATE INDEX IF NOT EXISTS idx_frames_object   ON frames(object);
		CREATE INDEX IF NOT EXISTS idx_frames_filter   ON frames(filter);
		CREATE INDEX IF NOT EXISTS idx_frames_date_obs ON frames(date_obs);
		CREATE INDEX IF NOT EXISTS idx_frames_obj_filt ON frames(object, filter);
		CREATE INDEX IF NOT EXISTS idx_frames_rejected ON frames(rejected);
	`)
	return err
}
