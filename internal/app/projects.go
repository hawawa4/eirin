package app

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"log/slog"

	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/siril"
	"github.com/TaruDesigns/eirin/internal/store"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Project is the frontend-facing project type.
type Project struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Folder      string `json:"folder"`
	CreatedAt   string `json:"createdAt"`
}

// GetProjectsFolder returns the configured projects root folder.
func (a *App) GetProjectsFolder() string {
	return a.store.Load().ProjectsFolder
}

// SelectProjectsFolder opens a directory picker for the projects root.
func (a *App) SelectProjectsFolder() (string, error) {
	path, err := a.wails.Dialog.OpenFile().
		SetTitle("Select Projects Root Folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	return path, nil
}

// SetProjectsFolder persists the projects root folder preference.
func (a *App) SetProjectsFolder(path string) error {
	return a.store.Set(store.KeyProjectsFolder, path)
}

// ListProjects returns all projects from the database.
func (a *App) ListProjects() ([]Project, error) {
	rows, err := a.store.ListProjects()
	if err != nil {
		return nil, err
	}
	projects := make([]Project, len(rows))
	for i, r := range rows {
		projects[i] = Project{
			ID:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Folder:      r.Folder,
			CreatedAt:   r.CreatedAt,
		}
	}
	return projects, nil
}

// CreateProject creates a new project: DB record + folder + lights subfolder.
func (a *App) CreateProject(name, description string) (Project, error) {
	pf := a.store.Load().ProjectsFolder
	if pf == "" {
		return Project{}, fmt.Errorf("projects folder not configured — set it in Settings first")
	}
	folderName := sanitizeFolderName(name)
	folder := filepath.Join(pf, folderName)
	if err := os.MkdirAll(filepath.Join(folder, "lights"), 0o750); err != nil {
		return Project{}, fmt.Errorf("creating project folder: %w", err)
	}
	row, err := a.store.CreateProject(name, description, folder)
	if err != nil {
		return Project{}, err
	}
	return Project{
		ID:          row.ID,
		Name:        row.Name,
		Description: row.Description,
		Folder:      row.Folder,
		CreatedAt:   row.CreatedAt,
	}, nil
}

// DeleteProject removes the project from the database (does not delete files).
func (a *App) DeleteProject(id int64) error {
	return a.store.DeleteProject(id)
}

// AddFramesToProject symlinks or copies NAS frames into <projectFolder>/lights/.
// mode must be "symlink" or "copy".
func (a *App) AddFramesToProject(projectFolder string, nasPaths []string, mode string) error {
	lightsDir := filepath.Join(projectFolder, "lights")
	if err := os.MkdirAll(lightsDir, 0o750); err != nil {
		return fmt.Errorf("creating lights dir: %w", err)
	}
	for _, src := range nasPaths {
		dst := filepath.Join(lightsDir, filepath.Base(src))
		_ = os.Remove(dst) // overwrite if already present
		var err error
		switch mode {
		case "symlink":
			err = os.Symlink(src, dst)
		case "copy":
			err = copyFileProject(src, dst)
		default:
			return fmt.Errorf("unknown mode %q — use symlink or copy", mode)
		}
		if err != nil {
			return fmt.Errorf("adding %s: %w", filepath.Base(src), err)
		}
	}
	return nil
}

// OpenProjectInSiril launches Siril with the project folder as working directory.
func (a *App) OpenProjectInSiril(projectFolder string) error {
	exe := a.sirilExecutable()
	cmd := exec.Command(exe, "-d", projectFolder)
	cmd.Env = siril.GUIEnv()
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// ProjectOutputFile describes a file in the project root (outside lights/).
type ProjectOutputFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// GetProjectOutputFiles lists files directly in the project root folder,
// skipping all subdirectories (including lights/). These are the processed
// outputs created by Siril.
func (a *App) GetProjectOutputFiles(projectFolder string) ([]ProjectOutputFile, error) {
	entries, err := os.ReadDir(projectFolder)
	if err != nil {
		if os.IsNotExist(err) {
			return []ProjectOutputFile{}, nil
		}
		return nil, err
	}
	files := make([]ProjectOutputFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, ProjectOutputFile{
			Name:    e.Name(),
			Path:    filepath.Join(projectFolder, e.Name()),
			Size:    info.Size(),
			ModTime: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return files, nil
}

// ImportOutputFiles copies project output files to destFolder on the NAS,
// then indexes each copied file so it appears in the library immediately.
// Metadata (object, telescope, instrument) is inherited from the first light
// symlink found in the project's lights/ folder.
func (a *App) ImportOutputFiles(filePaths []string, destFolder string) error {
	if err := os.MkdirAll(destFolder, 0o750); err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}

	// Derive project folder from the first source file path and read light metadata.
	meta := store.DirMeta{}
	if len(filePaths) > 0 {
		projectFolder := filepath.Dir(filePaths[0])
		lightsDir := filepath.Join(projectFolder, "lights")
		if entries, err := os.ReadDir(lightsDir); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				linkPath := filepath.Join(lightsDir, e.Name())
				nasPath, err := filepath.EvalSymlinks(linkPath)
				if err != nil {
					nasPath = linkPath
				}
				frames, err := a.store.GetFrames([]string{nasPath})
				if err == nil {
					if f, ok := frames[nasPath]; ok {
						meta = store.DirMeta{
							Object:     f.Object,
							Telescope:  f.Telescope,
							Instrument: f.Instrument,
							Filter:     f.Filter,
						}
						break
					}
				}
			}
		}
	}

	copied := 0
	for _, src := range filePaths {
		dst := filepath.Join(destFolder, filepath.Base(src))
		if err := copyFileProject(src, dst); err != nil {
			return fmt.Errorf("copying %s: %w", filepath.Base(src), err)
		}
		info, err := os.Stat(src)
		if err == nil {
			a.indexImportedFileWithMeta(importer.Candidate{
				SourcePath:   src,
				RelativePath: filepath.Base(src),
				DestPath:     dst,
				FileSize:     info.Size(),
			}, meta)
		}
		copied++
	}
	if copied > 0 {
		a.wails.Event.EmitEvent(&application.CustomEvent{Name: "library:updated"})
	}
	return nil
}

// GetProjectFrames lists filenames present in <projectFolder>/lights/.
func (a *App) GetProjectFrames(projectFolder string) ([]string, error) {
	lightsDir := filepath.Join(projectFolder, "lights")
	entries, err := os.ReadDir(lightsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// GetProjectLibraryFrames returns full LibraryFrame metadata for every frame in
// <projectFolder>/lights/, resolving symlinks back to the NAS source path for
// the DB lookup. Frames not yet in the DB are returned with just their filename.
func (a *App) GetProjectLibraryFrames(projectFolder string) []LibraryFrame {
	lightsDir := filepath.Join(projectFolder, "lights")
	entries, err := os.ReadDir(lightsDir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("project frames: readdir", "dir", lightsDir, "err", err)
		}
		return []LibraryFrame{}
	}

	type resolved struct {
		name    string
		nasPath string
	}
	items := make([]resolved, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		linkPath := filepath.Join(lightsDir, e.Name())
		realPath, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			realPath = linkPath
		}
		items = append(items, resolved{name: e.Name(), nasPath: realPath})
	}

	nasPaths := make([]string, len(items))
	for i, it := range items {
		nasPaths[i] = it.nasPath
	}

	frameMap, err := a.store.GetFrames(nasPaths)
	if err != nil {
		slog.Warn("project frames: db lookup", "err", err)
		frameMap = map[string]store.Frame{}
	}

	result := make([]LibraryFrame, 0, len(items))
	for _, it := range items {
		if f, ok := frameMap[it.nasPath]; ok {
			result = append(result, toLibraryFrame(f))
		} else {
			result = append(result, LibraryFrame{NasPath: it.nasPath, FileName: it.name})
		}
	}
	return result
}

// RemoveFramesFromProject removes the file or symlink in <projectFolder>/lights/
// whose basename matches each given NAS path.
func (a *App) RemoveFramesFromProject(projectFolder string, nasPaths []string) error {
	lightsDir := filepath.Join(projectFolder, "lights")
	failed := 0
	for _, p := range nasPaths {
		target := filepath.Join(lightsDir, filepath.Base(p))
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			slog.Warn("remove from project", "file", filepath.Base(p), "err", err)
			failed++
		}
	}
	if failed > 0 {
		return fmt.Errorf("failed to remove %d file(s) from project", failed)
	}
	return nil
}

var reSafeFolder = regexp.MustCompile(`[^\w\-]+`)

func sanitizeFolderName(name string) string {
	safe := reSafeFolder.ReplaceAllString(strings.TrimSpace(name), "_")
	safe = strings.Trim(safe, "_")
	if safe == "" {
		safe = "project"
	}
	return safe
}

func copyFileProject(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
