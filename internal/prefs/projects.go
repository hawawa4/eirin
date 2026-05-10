package prefs

import (
	"time"

	sq "github.com/Masterminds/squirrel"
)

// ProjectRow is the raw DB representation of a project.
type ProjectRow struct {
	ID          int64
	Name        string
	Description string
	Folder      string
	CreatedAt   string
}

// ListProjects returns all projects ordered newest first.
func (s *Store) ListProjects() ([]ProjectRow, error) {
	query, args, err := s.qb.
		Select("id", "name", "description", "folder", "created_at").
		From("projects").
		OrderBy("created_at DESC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []ProjectRow
	for rows.Next() {
		var p ProjectRow
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Folder, &p.CreatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if projects == nil {
		projects = []ProjectRow{}
	}
	return projects, rows.Err()
}

// CreateProject inserts a new project record and returns it.
func (s *Store) CreateProject(name, description, folder string) (ProjectRow, error) {
	createdAt := time.Now().UTC().Format(time.RFC3339)
	query, args, err := s.qb.
		Insert("projects").
		Columns("name", "description", "folder", "created_at").
		Values(name, description, folder, createdAt).
		ToSql()
	if err != nil {
		return ProjectRow{}, err
	}
	result, err := s.db.Exec(query, args...)
	if err != nil {
		return ProjectRow{}, err
	}
	id, _ := result.LastInsertId()
	return ProjectRow{
		ID:          id,
		Name:        name,
		Description: description,
		Folder:      folder,
		CreatedAt:   createdAt,
	}, nil
}

// DeleteProject removes a project by ID (does not touch the filesystem).
func (s *Store) DeleteProject(id int64) error {
	query, args, err := s.qb.
		Delete("projects").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return err
	}
	_, err = s.db.Exec(query, args...)
	return err
}
