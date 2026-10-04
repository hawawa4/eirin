package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"unicode/utf8"

	sq "github.com/Masterminds/squirrel"
)

// Helpers for library snapshots: the desktop app copies its database onto the
// NAS (VacuumInto), and the headless server turns a copy of it into a
// read-only viewer database (RewriteRoot, PruneToFinals, Compact, OpenReadOnly).

// LatestSchemaVersion is the highest migration version this build knows about.
// A database with a newer version was written by a newer Eirin.
func LatestSchemaVersion() int {
	return migrations[len(migrations)-1].version
}

// SchemaVersion returns the highest migration version applied to the database.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v)
	return v, err
}

// VacuumInto writes a consistent, compacted copy of the database to path,
// including changes still pending in the WAL. An existing file at path is
// replaced (VACUUM INTO itself refuses to overwrite).
func (s *Store) VacuumInto(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	_, err := s.db.Exec(`VACUUM INTO ?`, path)
	return err
}

// ChangeCount returns the number of rows changed through this store since it
// was opened. The store holds a single connection, so comparing two readings
// tells whether anything was written in between.
func (s *Store) ChangeCount() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT total_changes()`).Scan(&n)
	return n, err
}

// OpenReadOnly opens an existing database without creating it or running
// migrations; every write fails. The file must use a rollback journal (see
// Compact), since a WAL database can't be opened without its -shm file.
func OpenReadOnly(path string) (*Store, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=query_only(1)"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open read-only: %w", err)
	}
	qb := sq.StatementBuilder.PlaceholderFormat(sq.Question)
	return &Store{db: db, qb: qb, path: path}, nil
}

// RewriteRoot moves every frame from under oldRoot to the same relative path
// under newRoot, drops frames that aren't under oldRoot, and points the
// root_folder preference at newRoot. Used when a snapshot taken on one machine
// is served from another where the library is mounted somewhere else.
func (s *Store) RewriteRoot(oldRoot, newRoot string) error {
	oldPrefix := strings.TrimRight(oldRoot, "/") + "/"
	newPrefix := strings.TrimRight(newRoot, "/") + "/"
	// SQLite's substr counts characters, not bytes.
	oldLen := utf8.RuneCountInString(oldPrefix)

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`DELETE FROM frames WHERE substr(nas_path, 1, ?) <> ?`, oldLen, oldPrefix,
	); err != nil {
		return fmt.Errorf("drop frames outside root: %w", err)
	}
	if oldPrefix != newPrefix {
		if _, err := tx.Exec(
			`UPDATE frames SET nas_path = ? || substr(nas_path, ?)`, newPrefix, oldLen+1,
		); err != nil {
			return fmt.Errorf("rewrite paths: %w", err)
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO preferences (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		KeyRootFolder, strings.TrimRight(newRoot, "/"),
	); err != nil {
		return fmt.Errorf("set root folder: %w", err)
	}
	return tx.Commit()
}

// PruneToFinals reduces the database to what the read-only viewer shows:
// non-rejected stacked, processed and raster images. Subs and calibration
// frames, projects, and every preference except the root folder are removed.
func (s *Store) PruneToFinals() error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmts := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM frames WHERE frame_type NOT IN (?, ?, ?) OR rejected <> 0`,
			[]any{FrameTypeStacked, FrameTypeProcessed, FrameTypeImage}},
		{`DELETE FROM projects`, nil},
		{`DELETE FROM preferences WHERE key <> ?`, []any{KeyRootFolder}},
	}
	for _, st := range stmts {
		if _, err := tx.Exec(st.sql, st.args...); err != nil {
			return fmt.Errorf("prune: %w", err)
		}
	}
	return tx.Commit()
}

// Compact reclaims free space and switches the database to a rollback
// journal, so the file can later be opened with OpenReadOnly on its own.
func (s *Store) Compact() error {
	if _, err := s.db.Exec(`VACUUM`); err != nil {
		return err
	}
	_, err := s.db.Exec(`PRAGMA journal_mode=DELETE`)
	return err
}
