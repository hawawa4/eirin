package prefs

import "testing"

func TestListProjectsEmpty(t *testing.T) {
	s := newTestStore(t)
	rows, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects (empty): %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 projects, got %d", len(rows))
	}
}

func TestCreateProject(t *testing.T) {
	s := newTestStore(t)

	row, err := s.CreateProject("Deep Sky Survey", "Ha + OIII", "/projects/deep-sky")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if row.ID == 0 {
		t.Error("ID should be non-zero after create")
	}
	if row.Name != "Deep Sky Survey" {
		t.Errorf("Name = %q, want Deep Sky Survey", row.Name)
	}
	if row.Description != "Ha + OIII" {
		t.Errorf("Description = %q, want Ha + OIII", row.Description)
	}
	if row.Folder != "/projects/deep-sky" {
		t.Errorf("Folder = %q, want /projects/deep-sky", row.Folder)
	}
	if row.CreatedAt == "" {
		t.Error("CreatedAt should be set")
	}
}

func TestListProjectsAfterCreate(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateProject("P1", "", "/p1"); err != nil {
		t.Fatalf("CreateProject P1: %v", err)
	}
	if _, err := s.CreateProject("P2", "", "/p2"); err != nil {
		t.Fatalf("CreateProject P2: %v", err)
	}

	rows, err := s.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 projects, got %d", len(rows))
	}
	// Verify both names are present (ordering by created_at may be unstable
	// when both rows are inserted within the same second).
	names := make(map[string]bool, len(rows))
	for _, r := range rows {
		names[r.Name] = true
	}
	if !names["P1"] || !names["P2"] {
		t.Errorf("expected P1 and P2 in listing, got %v", names)
	}
}

func TestDeleteProject(t *testing.T) {
	s := newTestStore(t)

	row, _ := s.CreateProject("To Delete", "", "/delete-me")
	if err := s.DeleteProject(row.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}

	rows, _ := s.ListProjects()
	if len(rows) != 0 {
		t.Errorf("expected 0 projects after delete, got %d", len(rows))
	}
}

func TestDeleteProjectUnknownIDIsNoop(t *testing.T) {
	s := newTestStore(t)
	// Deleting a non-existent ID should not error.
	if err := s.DeleteProject(99999); err != nil {
		t.Errorf("DeleteProject(unknown): %v", err)
	}
}

func TestCreateProjectDuplicateFolderFails(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.CreateProject("P1", "", "/projects/same"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := s.CreateProject("P2", "", "/projects/same"); err == nil {
		t.Error("expected error for duplicate folder constraint, got nil")
	}
}
