package prefs

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
)

// Frame type constants.
const (
	FrameTypeLight     = "light"
	FrameTypeDark      = "dark"
	FrameTypeFlat      = "flat"
	FrameTypeBias      = "bias"
	FrameTypeStacked   = "stacked"
	FrameTypeProcessed = "processed"
)

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

	// Classified frame type (auto-detected on index, can be overridden by user)
	FrameType string

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

// ClassifyFrameType determines the frame type from the file path.
// The parent directory is checked first (folders ending in "_sub" indicate lights),
// then the filename prefix is used.
func ClassifyFrameType(nasPath string) string {
	name := filepath.Base(nasPath)
	dir := filepath.Base(filepath.Dir(nasPath))

	if strings.HasSuffix(dir, "_sub") {
		return FrameTypeLight
	}
	switch {
	case strings.HasPrefix(name, "Light"):
		return FrameTypeLight
	case strings.HasPrefix(name, "Dark"):
		return FrameTypeDark
	case strings.HasPrefix(name, "Flat"):
		return FrameTypeFlat
	case strings.HasPrefix(strings.ToLower(name), "bias"):
		return FrameTypeBias
	default:
		return FrameTypeStacked
	}
}

// UpsertFrame stores or updates the FITS header fields for a single frame.
// Only the header-derived columns are written; user metadata (rejected, tags, etc.)
// and analysis results (wcs, quality) are preserved if the row already exists.
func (s *Store) UpsertFrame(path string, f Frame) error {
	query, args, err := s.qb.
		Insert("frames").
		Columns("nas_path", "file_size", "last_seen", "cached_at",
			"object", "filter", "exptime", "gain", "ccd_temp", "date_obs",
			"telescope", "instrument", "frame_type").
		Values(path, f.FileSize, f.LastSeen, time.Now().Unix(),
			f.Object, f.Filter, f.ExpTime, f.Gain, f.CCDTemp, f.DateObs,
			f.Telescope, f.Instrument, f.FrameType).
		Suffix(`ON CONFLICT(nas_path) DO UPDATE SET
			file_size=excluded.file_size, last_seen=excluded.last_seen,
			cached_at=excluded.cached_at,
			object=excluded.object, filter=excluded.filter, exptime=excluded.exptime,
			gain=excluded.gain, ccd_temp=excluded.ccd_temp, date_obs=excluded.date_obs,
			telescope=excluded.telescope, instrument=excluded.instrument,
			frame_type=excluded.frame_type`).
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
			 object, filter, exptime, gain, ccd_temp, date_obs, telescope, instrument,
			 frame_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(nas_path) DO UPDATE SET
			file_size=excluded.file_size, last_seen=excluded.last_seen,
			cached_at=excluded.cached_at,
			object=excluded.object, filter=excluded.filter, exptime=excluded.exptime,
			gain=excluded.gain, ccd_temp=excluded.ccd_temp, date_obs=excluded.date_obs,
			telescope=excluded.telescope, instrument=excluded.instrument,
			frame_type=excluded.frame_type
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for path, f := range entries {
		if _, err := stmt.Exec(path, f.FileSize, f.LastSeen, now,
			f.Object, f.Filter, f.ExpTime, f.Gain, f.CCDTemp, f.DateObs,
			f.Telescope, f.Instrument, f.FrameType); err != nil {
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

// GetAllFramesUnder returns all indexed (cached_at > 0) frames whose path
// starts with rootPath. Results are sorted by object then date_obs.
func (s *Store) GetAllFramesUnder(rootPath string) ([]Frame, error) {
	prefix := rootPath
	if len(prefix) > 0 && prefix[len(prefix)-1] != '/' {
		prefix += "/"
	}
	query, args, err := s.qb.
		Select(
			"nas_path", "file_size", "last_seen", "cached_at",
			"object", "filter", "exptime", "gain", "ccd_temp", "date_obs", "telescope", "instrument",
			"frame_type",
			"ra", "dec", "pixel_scale", "rotation", "wcs_solved",
			"fwhm", "fwhm_unit", "roundness", "background", "noise", "snr", "star_count", "quality_analyzed",
			"approved", "rejected", "rejection_reason", "tags", "notes",
		).
		From("frames").
		Where(sq.And{
			sq.Like{"nas_path": prefix + "%"},
			sq.Gt{"cached_at": 0},
		}).
		OrderBy("object", "date_obs").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var frames []Frame
	for rows.Next() {
		_, f, err := scanFrame(rows)
		if err != nil {
			return nil, err
		}
		frames = append(frames, f)
	}
	return frames, rows.Err()
}

// SetFrameType updates the frame_type for the given path. This allows the user
// to manually override the auto-detected type (e.g. to mark a file as "processed").
func (s *Store) SetFrameType(path, frameType string) error {
	_, err := s.db.Exec(`UPDATE frames SET frame_type=? WHERE nas_path=?`, frameType, path)
	return err
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

// GetAllFrameBasenames returns a set of all file basenames currently in the
// frames table. Used by the importer to identify files already in the library.
func (s *Store) GetAllFrameBasenames() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT nas_path FROM frames`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]bool)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		result[filepath.Base(p)] = true
	}
	return result, rows.Err()
}

// FrameQuality holds quality metrics produced by Siril headless analysis.
type FrameQuality struct {
	FWHM       float64
	FWHMUnit   string // "px" or "arcsec"
	Background float64
	Noise      float64
	SNR        float64 // derived as Background/Noise
	StarCount  int64
}

// UpdateFrameQuality persists Siril quality metrics for a frame.
// The row must already exist (frames are created by the indexer).
func (s *Store) UpdateFrameQuality(nasPath string, q FrameQuality) error {
	_, err := s.db.Exec(`
		UPDATE frames SET
			fwhm=?, fwhm_unit=?, background=?, noise=?, snr=?,
			star_count=?, quality_analyzed=1
		WHERE nas_path=?
	`, q.FWHM, q.FWHMUnit, q.Background, q.Noise, q.SNR,
		q.StarCount, nasPath)
	return err
}

// WCSResult holds plate-solve coordinates produced by Siril.
type WCSResult struct {
	RA         float64
	Dec        float64
	PixelScale float64 // arcsec/pixel
	Rotation   float64 // degrees
}

// UpdateWCS persists plate-solve results for a frame.
func (s *Store) UpdateWCS(nasPath string, r WCSResult) error {
	_, err := s.db.Exec(`
		UPDATE frames SET
			ra=?, dec=?, pixel_scale=?, rotation=?, wcs_solved=1
		WHERE nas_path=?
	`, r.RA, r.Dec, r.PixelScale, r.Rotation, nasPath)
	return err
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

func (s *Store) getFramesChunk(paths []string, out map[string]Frame) error {
	query, args, err := s.qb.
		Select(
			"nas_path", "file_size", "last_seen", "cached_at",
			"object", "filter", "exptime", "gain", "ccd_temp", "date_obs", "telescope", "instrument",
			"frame_type",
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
		frameType                                      string

		ra, dec, pixScale, rotation             sql.NullFloat64
		wcsSolved                               int64
		fwhm, roundness, background, noise, snr sql.NullFloat64
		fwhmUnit                                sql.NullString
		starCount                               sql.NullInt64
		qualityAnalyzed                         int64

		approved        sql.NullInt64
		rejected        int64
		rejectionReason sql.NullString
		tags, notes     sql.NullString
	)

	err := rows.Scan(
		&path, &fileSize, &lastSeen, &cachedAt,
		&object, &filter, &exptime, &gain, &ccdTemp, &dateObs, &telescope, &instrument,
		&frameType,
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
		FrameType:       frameType,
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
