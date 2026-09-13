//go:build server

package app

// SuggestResult is a frame flagged as a statistical outlier within its group.
// Kept here too so the type is always part of the Wails binding surface
// regardless of build tag.
type SuggestResult struct {
	Frame       LibraryFrame `json:"frame"`
	GroupMedian float64      `json:"groupMedian"`
	GroupSigma  float64      `json:"groupSigma"`
	Sigmas      float64      `json:"sigmas"`
}

// SuggestRejects always returns empty in headless/server builds — quality
// analysis (which populates the data this relies on) requires the Siril CLI,
// which is not available there.
func (a *App) SuggestRejects(rootPath string, threshold float64) []SuggestResult {
	return nil
}
