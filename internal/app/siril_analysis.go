package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/TaruDesigns/eirin/internal/prefs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// AnalysisProgress is emitted as "analysis:progress" during frame analysis.
type AnalysisProgress struct {
	Phase   string `json:"phase"`   // "analyzing" | "done" | "cancelled"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Current string `json:"current"` // current file basename
	Errors  int    `json:"errors"`
}

var (
	analysisMu     sync.Mutex
	analysisCancel context.CancelFunc
)

// AnalyzeFrames runs Siril headless quality analysis (findstar) on each NAS path
// sequentially. It emits "analysis:progress" events throughout and
// "library:updated" on completion so the frontend can refresh.
func (a *App) AnalyzeFrames(nasPaths []string) error {
	if len(nasPaths) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	analysisMu.Lock()
	if analysisCancel != nil {
		analysisCancel()
	}
	analysisCancel = cancel
	analysisMu.Unlock()
	defer func() {
		analysisMu.Lock()
		analysisCancel = nil
		analysisMu.Unlock()
		cancel()
	}()

	exe := a.sirilCliExecutable()
	total := len(nasPaths)
	errCount := 0

	for i, path := range nasPaths {
		select {
		case <-ctx.Done():
			a.emitAnalysisProgress("cancelled", total, i, "", errCount)
			return nil
		default:
		}
		a.emitAnalysisProgress("analyzing", total, i, filepath.Base(path), errCount)

		quality, err := analyzeSingleFrame(ctx, exe, path)
		if err != nil {
			runtime.LogWarningf(a.ctx, "analysis: %v", err)
			errCount++
		} else if err := a.prefs.UpdateFrameQuality(path, quality); err != nil {
			runtime.LogWarningf(a.ctx, "analysis: db update: %v", err)
			errCount++
		}
	}

	a.emitAnalysisProgress("done", total, total, "", errCount)
	runtime.EventsEmit(a.ctx, "library:updated")
	return nil
}

// CancelAnalysis interrupts an in-progress AnalyzeFrames call.
func (a *App) CancelAnalysis() {
	analysisMu.Lock()
	if analysisCancel != nil {
		analysisCancel()
	}
	analysisMu.Unlock()
}

func (a *App) emitAnalysisProgress(phase string, total, done int, current string, errors int) {
	runtime.EventsEmit(a.ctx, "analysis:progress", AnalysisProgress{
		Phase:   phase,
		Total:   total,
		Done:    done,
		Current: current,
		Errors:  errors,
	})
}

// sirilCliExecutable returns the best available headless Siril binary.
// Prefers siril-cli (no GUI required); falls back to the configured siril path.
func (a *App) sirilCliExecutable() string {
	configured := a.prefs.Load().SirilPath
	if configured != "" {
		cliPath := filepath.Join(filepath.Dir(configured), "siril-cli")
		if _, err := os.Stat(cliPath); err == nil {
			return cliPath
		}
		return configured
	}
	if p, err := exec.LookPath("siril-cli"); err == nil {
		return p
	}
	return "siril"
}

// analyzeSingleFrame writes a minimal Siril script, runs it headlessly,
// and parses the findstar output for quality metrics.
func analyzeSingleFrame(ctx context.Context, exe, nasPath string) (prefs.FrameQuality, error) {
	escaped := strings.ReplaceAll(nasPath, `"`, `\"`)
	script := "requires 1.0.0\nload \"" + escaped + "\"\nfindstar\n"

	tmp, err := os.CreateTemp("", "eirin-siril-*.ssf")
	if err != nil {
		return prefs.FrameQuality{}, fmt.Errorf("tmp script: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return prefs.FrameQuality{}, fmt.Errorf("write script: %w", err)
	}
	tmp.Close()

	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, exe, "-s", tmp.Name())
	output, _ := cmd.CombinedOutput() // Siril may exit non-zero even on success

	if len(output) == 0 {
		return prefs.FrameQuality{}, fmt.Errorf("no output for %s", filepath.Base(nasPath))
	}
	q, err := parseFindstarOutput(string(output))
	if err != nil {
		return prefs.FrameQuality{}, fmt.Errorf("%s: %w", filepath.Base(nasPath), err)
	}
	return q, nil
}

// ── Output parsers ─────────────────────────────────────────────────────────

var (
	// "found N stars" or "N stars found/detected"
	reStarCount = regexp.MustCompile(`(?i)\b(\d+)\s+stars?\b`)

	// FWHM value optionally followed by " (arcsec) or px/pixel
	// Handles: "FWHM: 2.34\"", "FWHM(x)=2.34 px", "Average FWHM: 2.34"
	reFWHM = regexp.MustCompile(`(?i)\bfwhm\b[^0-9\n]{0,25}?([0-9]+(?:[.,][0-9]+)?)\s*(")?`)

	reRoundness  = regexp.MustCompile(`(?i)\broundness\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
	reBackground = regexp.MustCompile(`(?i)\bbackground\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
	reNoise      = regexp.MustCompile(`(?i)\bnoise\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
	reSNR        = regexp.MustCompile(`(?i)\bsnr\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
)

func parseFindstarOutput(output string) (prefs.FrameQuality, error) {
	var q prefs.FrameQuality
	found := false

	for _, line := range strings.Split(output, "\n") {
		if m := reStarCount.FindStringSubmatch(line); m != nil {
			if v, err := strconv.ParseInt(m[1], 10, 64); err == nil && v > 0 {
				q.StarCount = v
				found = true
			}
		}
		if q.FWHM == 0 {
			if m := reFWHM.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil && v > 0 {
					q.FWHM = v
					if m[2] == `"` {
						q.FWHMUnit = "arcsec"
					} else {
						q.FWHMUnit = "px"
					}
					found = true
				}
			}
		}
		if q.Roundness == 0 {
			if m := reRoundness.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					q.Roundness = v
					found = true
				}
			}
		}
		if q.Background == 0 {
			if m := reBackground.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					q.Background = v
					found = true
				}
			}
		}
		if q.Noise == 0 {
			if m := reNoise.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					q.Noise = v
					found = true
				}
			}
		}
		if q.SNR == 0 {
			if m := reSNR.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					q.SNR = v
					found = true
				}
			}
		}
	}

	if !found {
		return q, fmt.Errorf("no recognisable findstar output")
	}
	return q, nil
}

// parseDecimal handles both "." and "," as decimal separator.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
}
