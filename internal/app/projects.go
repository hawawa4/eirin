package app

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"log/slog"

	"github.com/TaruDesigns/eirin/internal/importer"
	"github.com/TaruDesigns/eirin/internal/projectfs"
	"github.com/TaruDesigns/eirin/internal/store"
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

// CreateProject creates a new project: DB record + folder with lights/,
// darks/, flats/ and biases/ subfolders. It fails if the sanitized folder
// already exists on disk (e.g. "M31 Ha" and "M31_Ha" map to the same folder).
func (a *App) CreateProject(name, description string) (Project, error) {
	pf := a.store.Load().ProjectsFolder
	if pf == "" {
		return Project{}, fmt.Errorf("projects folder not configured — set it in Settings first")
	}
	folder := filepath.Join(pf, sanitizeFolderName(name))
	if err := projectfs.Create(folder); err != nil {
		return Project{}, err
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

// AddFramesResult summarises an AddFramesToProjectDetailed call.
type AddFramesResult struct {
	Added          int `json:"added"`          // new entries created
	AlreadyPresent int `json:"alreadyPresent"` // same source already in the project (no-op)
	// Skipped lists NAS paths not added because their frame type (stacked,
	// processed, image, unknown) has no place in a Siril project.
	Skipped []string `json:"skipped"`
}

// AddFramesToProject symlinks or copies NAS frames into the project, routing
// each into lights/, darks/, flats/ or biases/ by its frame type. mode must be
// "symlink" or "copy". Frames of other types are skipped (and logged); an
// error is returned only if frames were requested but none could be placed.
// See AddFramesToProjectDetailed for per-call counts.
func (a *App) AddFramesToProject(projectFolder string, nasPaths []string, mode string) error {
	res, err := a.AddFramesToProjectDetailed(projectFolder, nasPaths, mode)
	if err != nil {
		return err
	}
	if len(res.Skipped) > 0 && res.Added == 0 && res.AlreadyPresent == 0 {
		return fmt.Errorf("none of the selected frames can be added: only light, dark, flat and bias frames belong in a project (%d skipped)", len(res.Skipped))
	}
	return nil
}

// AddFramesToProjectDetailed is AddFramesToProject returning what happened.
// An entry with the same name that refers to the same source is left alone;
// one that refers to a different source gets a numeric suffix (name_2.fit).
func (a *App) AddFramesToProjectDetailed(projectFolder string, nasPaths []string, mode string) (AddFramesResult, error) {
	res := AddFramesResult{Skipped: []string{}}
	if !projectfs.ValidMode(mode) {
		return res, fmt.Errorf("unknown mode %q — use symlink or copy", mode)
	}
	frames, err := a.store.GetFrames(nasPaths)
	if err != nil {
		return res, fmt.Errorf("looking up frame types: %w", err)
	}
	for _, src := range nasPaths {
		frameType := store.ClassifyFrameType(src) // fallback for frames not yet indexed
		if f, ok := frames[src]; ok {
			frameType = f.FrameType
		}
		sub, ok := projectfs.SubdirFor(frameType)
		if !ok {
			slog.Info("add to project: skipping unsupported frame type", "path", src, "frameType", frameType)
			res.Skipped = append(res.Skipped, src)
			continue
		}
		r, err := projectfs.AddFrame(projectFolder, sub, src, mode)
		if err != nil {
			return res, fmt.Errorf("adding %s: %w", filepath.Base(src), err)
		}
		if r.Existed {
			res.AlreadyPresent++
		} else {
			res.Added++
		}
	}
	return res, nil
}

// ProjectOutputFile describes a file in the project root (outside the frame subfolders).
type ProjectOutputFile struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
}

// GetProjectOutputFiles lists files directly in the project root folder,
// skipping all subdirectories (including lights/, darks/, flats/, biases/ and
// Siril's process/). These are the processed outputs created by Siril.
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
		// Skip every subfolder, and the frame subfolders even if they are
		// symlinks to directories.
		if e.IsDir() || projectfs.IsFrameSubdir(e.Name()) {
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

// ImportOutputsResult is returned by ImportOutputFiles.
type ImportOutputsResult struct {
	// Copied lists the file names copied (and indexed) into the destination.
	Copied []string `json:"copied"`
	// Conflicts lists file names that already exist in the destination (or
	// appear twice in the request). When non-empty, nothing was copied.
	Conflicts []string `json:"conflicts"`
}

// ImportOutputFiles copies project output files to destFolder on the NAS,
// then indexes each copied file so it appears in the library immediately.
//
// It is all-or-nothing with respect to name conflicts: existing NAS files are
// never overwritten, so if any destination already exists (or two sources
// share a name), nothing is copied and the result lists the conflicting names
// with a nil error. The error is reserved for real failures (invalid
// destination, I/O errors); a copy failing midway keeps the files copied so
// far (listed in Copied). Metadata (object, telescope, instrument, filter) is
// inherited from the first project light frame found in the DB.
func (a *App) ImportOutputFiles(filePaths []string, destFolder string) (ImportOutputsResult, error) {
	res := ImportOutputsResult{Copied: []string{}, Conflicts: []string{}}
	if err := a.validateOutputDest(destFolder); err != nil {
		return res, err
	}
	seen := map[string]bool{}
	for _, src := range filePaths {
		name := filepath.Base(src)
		if seen[name] {
			res.Conflicts = append(res.Conflicts, name)
			continue
		}
		seen[name] = true
		if _, err := os.Lstat(filepath.Join(destFolder, name)); err == nil {
			res.Conflicts = append(res.Conflicts, name)
		}
	}
	if len(res.Conflicts) > 0 {
		return res, nil
	}
	if err := os.MkdirAll(destFolder, 0o750); err != nil {
		return res, fmt.Errorf("creating destination: %w", err)
	}

	meta := store.DirMeta{}
	if len(filePaths) > 0 {
		meta = a.projectLightMeta(filepath.Dir(filePaths[0]))
	}
	defer func() {
		if len(res.Copied) > 0 {
			a.emitEvent("library:updated", nil)
		}
	}()

	for _, src := range filePaths {
		name := filepath.Base(src)
		dst := filepath.Join(destFolder, name)
		if err := projectfs.CopyNoOverwrite(src, dst); err != nil {
			return res, fmt.Errorf("copying %s: %w", name, err)
		}
		if info, err := os.Stat(src); err == nil {
			a.indexImportedFileWithMeta(importer.Candidate{
				SourcePath:   src,
				RelativePath: name,
				DestPath:     dst,
				FileSize:     info.Size(),
			}, meta)
		}
		res.Copied = append(res.Copied, name)
	}
	return res, nil
}

// validateOutputDest rejects destinations that are relative, contain ".."
// segments, or (when a library root is configured) lie outside the root.
func (a *App) validateOutputDest(destFolder string) error {
	if !filepath.IsAbs(destFolder) {
		return fmt.Errorf("destination must be an absolute path: %q", destFolder)
	}
	for _, seg := range strings.FieldsFunc(destFolder, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return fmt.Errorf("destination can't contain \"..\": %q", destFolder)
		}
	}
	if root := a.store.Load().RootFolder; root != "" {
		rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(destFolder))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("destination %q is outside the library root %q", destFolder, root)
		}
	}
	return nil
}

// projectLightMeta returns object/telescope/instrument/filter from the first
// light frame of the project (in any frame subfolder) that is in the DB.
func (a *App) projectLightMeta(projectFolder string) store.DirMeta {
	entries, _ := projectfs.ListFrames(projectFolder)
	paths := make([]string, len(entries))
	for i, e := range entries {
		paths[i] = e.SourcePath
	}
	frames, err := a.store.GetFrames(paths)
	if err != nil {
		return store.DirMeta{}
	}
	for _, e := range entries {
		if f, ok := frames[e.SourcePath]; ok && f.FrameType == store.FrameTypeLight {
			return store.DirMeta{Object: f.Object, Telescope: f.Telescope, Instrument: f.Instrument, Filter: f.Filter}
		}
	}
	return store.DirMeta{}
}

// GetProjectFrames lists the file names present in the project's frame
// subfolders (lights/, darks/, flats/, biases/).
func (a *App) GetProjectFrames(projectFolder string) ([]string, error) {
	entries, err := projectfs.ListFrames(projectFolder)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return names, nil
}

// GetProjectLibraryFrames returns full LibraryFrame metadata for every frame
// in the project's frame subfolders, resolving symlinks back to the NAS source
// path for the DB lookup. FrameType comes from the DB; frames not in the DB
// are returned with their filename and the type implied by their subfolder.
// Legacy projects with darks/biases inside lights/ are listed too.
func (a *App) GetProjectLibraryFrames(projectFolder string) ([]LibraryFrame, error) {
	entries, err := projectfs.ListFrames(projectFolder)
	if err != nil {
		slog.Warn("project frames: readdir", "dir", projectFolder, "err", err)
		return nil, fmt.Errorf("reading project folder: %w", err)
	}
	nasPaths := make([]string, len(entries))
	for i, e := range entries {
		nasPaths[i] = e.SourcePath
	}
	frameMap, err := a.store.GetFrames(nasPaths)
	if err != nil {
		slog.Warn("project frames: db lookup", "err", err)
		return nil, fmt.Errorf("reading library: %w", err)
	}

	result := make([]LibraryFrame, 0, len(entries))
	for _, e := range entries {
		if f, ok := frameMap[e.SourcePath]; ok {
			result = append(result, toLibraryFrame(f))
		} else {
			result = append(result, LibraryFrame{
				NasPath:   e.SourcePath,
				FileName:  e.Name,
				FrameType: projectfs.FrameTypeForSubdir(e.Subdir),
				MoonPhase: -1,
			})
		}
	}
	return result, nil
}

// RemoveFramesFromProject removes the project entries (symlinks or copies) in
// any frame subfolder that refer to the given NAS paths. NAS files are never
// touched.
func (a *App) RemoveFramesFromProject(projectFolder string, nasPaths []string) error {
	failed, err := projectfs.RemoveFrames(projectFolder, nasPaths)
	if err != nil {
		return err
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
