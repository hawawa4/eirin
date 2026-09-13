//go:build server

package app

import "errors"

// SelectSourceFolder is unavailable in headless/server builds — there is no
// native file dialog and no local Seestar device to import from.
func (a *App) SelectSourceFolder() (string, error) {
	return "", errors.New("importing is not available in server mode")
}

// ScanImportCandidates is unavailable in headless/server builds — import
// sources are local devices (e.g. a Seestar), which aren't reachable from
// inside a container.
func (a *App) ScanImportCandidates(sourceFolder string, extensions []string) ([]ImportCandidate, error) {
	return nil, errors.New("importing is not available in server mode")
}

// StartImport is unavailable in headless/server builds. See ScanImportCandidates.
func (a *App) StartImport(sourceFolder string, extensions []string, deleteAfterCopy bool) error {
	return errors.New("importing is not available in server mode")
}
