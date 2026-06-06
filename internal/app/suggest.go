package app

import (
	"math"
	"sort"

	"log/slog"

	"github.com/TaruDesigns/eirin/internal/store"
)

// SuggestResult is a frame flagged as a statistical outlier within its group.
type SuggestResult struct {
	Frame       LibraryFrame `json:"frame"`
	GroupMedian float64      `json:"groupMedian"` // median FWHM of the (object, filter) group
	GroupSigma  float64      `json:"groupSigma"`  // std dev FWHM of the group
	Sigmas      float64      `json:"sigmas"`      // how many σ above median this frame is
}

// SuggestRejects returns quality-analyzed light frames whose FWHM is more than
// threshold standard deviations above the median for their (object, filter) group.
// threshold defaults to 2.0 if ≤ 0.
func (a *App) SuggestRejects(rootPath string, threshold float64) []SuggestResult {
	if threshold <= 0 {
		threshold = 2.0
	}

	frames, err := a.store.GetAllFramesUnder(rootPath)
	if err != nil {
		slog.Error("suggest: get frames", "err", err)
		return nil
	}

	// Group quality-analyzed, non-rejected light frames by (object, filter).
	type groupKey struct{ object, filter string }
	groups := map[groupKey][]store.Frame{}
	for _, f := range frames {
		if f.FrameType != store.FrameTypeLight || !f.QualityAnalyzed || f.Rejected || f.FWHM == nil {
			continue
		}
		k := groupKey{f.Object, f.Filter}
		groups[k] = append(groups[k], f)
	}

	var results []SuggestResult
	for _, gFrames := range groups {
		if len(gFrames) < 4 {
			// Too few frames for meaningful statistics.
			continue
		}

		fwhms := make([]float64, len(gFrames))
		for i, f := range gFrames {
			fwhms[i] = *f.FWHM
		}
		median := medianFloat(fwhms)
		sigma := stdDevFloat(fwhms, median)
		if sigma == 0 {
			continue
		}

		for _, f := range gFrames {
			sigmas := (*f.FWHM - median) / sigma
			if sigmas > threshold {
				results = append(results, SuggestResult{
					Frame:       toLibraryFrame(f),
					GroupMedian: median,
					GroupSigma:  sigma,
					Sigmas:      sigmas,
				})
			}
		}
	}

	// Sort worst offenders first.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Sigmas > results[j].Sigmas
	})
	return results
}

func medianFloat(vals []float64) float64 {
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

func stdDevFloat(vals []float64, mean float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(vals)-1))
}
