//go:build server

package app

import "errors"

// AnalysisProgress is emitted as "analysis:progress" during frame analysis
// in desktop builds. Kept here too so the type is always part of the Wails
// binding surface regardless of build tag.
type AnalysisProgress struct {
	Phase   string `json:"phase"`
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Current string `json:"current"`
	Errors  int    `json:"errors"`
}

// AnalyzeFrames is unavailable in headless/server builds — analysis requires
// the Siril CLI, which is not part of the slim server image.
func (a *App) AnalyzeFrames(nasPaths []string) error {
	return errors.New("frame analysis is not available in server mode")
}

// CancelAnalysis is a no-op in headless/server builds since AnalyzeFrames
// never runs there.
func (a *App) CancelAnalysis() {}
