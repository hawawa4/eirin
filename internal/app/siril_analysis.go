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

// AnalyzeFrames runs Siril headless analysis (findstar + platesolve + statistics) on each
// NAS path sequentially. Raw output is written to $TMPDIR/eirin-siril-debug.txt.
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

	debugPath := filepath.Join(os.TempDir(), "eirin-siril-debug.txt")
	if df, err := os.OpenFile(debugPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644); err == nil {
		fmt.Fprintf(df, "=== eirin Siril debug log ===\nexe: %s\n", a.sirilCliExecutable())
		df.Close()
	}

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

		quality, wcs, wcsSolved, rawOutput, err := analyzeSingleFrame(ctx, exe, path)

		if df, ferr := os.OpenFile(debugPath, os.O_APPEND|os.O_WRONLY, 0644); ferr == nil {
			fmt.Fprintf(df, "\n--- %s ---\n%s\n", filepath.Base(path), string(rawOutput))
			df.Close()
		}
		runtime.LogInfof(a.ctx, "siril [%s]:\n%s", filepath.Base(path), string(rawOutput))

		if err != nil {
			runtime.LogWarningf(a.ctx, "analysis: %v", err)
			errCount++
		} else {
			if uerr := a.prefs.UpdateFrameQuality(path, quality); uerr != nil {
				runtime.LogWarningf(a.ctx, "analysis: db quality update: %v", uerr)
				errCount++
			}
			if wcsSolved {
				if uerr := a.prefs.UpdateWCS(path, wcs); uerr != nil {
					runtime.LogWarningf(a.ctx, "analysis: db wcs update: %v", uerr)
				}
			}
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

// analyzeSingleFrame runs findstar + platesolve + statistics for one frame.
func analyzeSingleFrame(ctx context.Context, exe, nasPath string) (prefs.FrameQuality, prefs.WCSResult, bool, []byte, error) {
	escaped := strings.ReplaceAll(nasPath, `"`, `\"`)
	script := "requires 1.0.0\nload \"" + escaped + "\"\nfindstar\nplatesolve\nstatistics\n"

	tmp, err := os.CreateTemp("", "eirin-siril-*.ssf")
	if err != nil {
		return prefs.FrameQuality{}, prefs.WCSResult{}, false, nil, fmt.Errorf("tmp script: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return prefs.FrameQuality{}, prefs.WCSResult{}, false, nil, fmt.Errorf("write script: %w", err)
	}
	tmp.Close()

	// 120s covers plate-solve catalog lookup time.
	runCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, exe, "-s", tmp.Name())
	output, _ := cmd.CombinedOutput() // Siril exits non-zero even on success

	if len(output) == 0 {
		return prefs.FrameQuality{}, prefs.WCSResult{}, false, output, fmt.Errorf("no output for %s", filepath.Base(nasPath))
	}
	q, err := parseSirilOutput(string(output))
	if err != nil {
		return prefs.FrameQuality{}, prefs.WCSResult{}, false, output, fmt.Errorf("%s: %w", filepath.Base(nasPath), err)
	}
	wcs, wcsSolved := parsePlateSolveOutput(string(output))
	return q, wcs, wcsSolved, output, nil
}

// ── Output parsers ─────────────────────────────────────────────────────────

var (
	// findstar: "Found 371 Gaussian profile stars in image, channel #0 (FWHM 3.383251)"
	reStarCount = regexp.MustCompile(`(?i)\bFound\s+(\d+)\s+\w+\s+\w+\s+stars?`)
	reFWHM      = regexp.MustCompile(`(?i)\bFWHM\s+([0-9]+(?:[.,][0-9]+)?)`)

	// statistics command output: median = background level, bgnoise/sigma = noise
	reMedian  = regexp.MustCompile(`(?i)\bmedian\b[^0-9\n]{0,15}([0-9]+(?:[.,][0-9]+)?)`)
	reBgNoise = regexp.MustCompile(`(?i)\bbgnoise\b[^0-9\n]{0,15}([0-9]+(?:[.,][0-9]+)?)`)
	reSigma   = regexp.MustCompile(`(?i)\bsigma\b[^0-9\n]{0,15}([0-9]+(?:[.,][0-9]+)?)`)

	// platesolve: "Image center: alpha: 06 45 51.505, delta: -20 46 52.259"
	rePlateAlpha = regexp.MustCompile(`(?i)\balpha\s*:\s*([0-9]{1,3})\s+([0-9]{1,2})\s+([0-9]+(?:[.,][0-9]+)?)`)
	rePlateDelta = regexp.MustCompile(`(?i)\bdelta\s*:\s*([+-]?[0-9]{1,3})\s+([0-9]{1,2})\s+([0-9]+(?:[.,][0-9]+)?)`)
	// "Resolution:      3.672 arcsec/px"
	rePlatePixScale = regexp.MustCompile(`(?i)(?:pixel\s*scale|resolution)\s*[=:]\s*([0-9]+(?:[.,][0-9]+)?)\s*arcsec`)
	// "Up is +180.17 deg CounterclockWise wrt. N"
	rePlateRotation = regexp.MustCompile(`(?i)\bup\s+is\s+([+-]?[0-9]+(?:[.,][0-9]+)?)\s*deg`)
)

// parseSirilOutput parses combined findstar + statistics output.
// SNR is derived as background/noise when both are available.
func parseSirilOutput(output string) (prefs.FrameQuality, error) {
	var q prefs.FrameQuality
	found := false

	for _, line := range strings.Split(output, "\n") {
		if q.StarCount == 0 {
			if m := reStarCount.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseInt(m[1], 10, 64); err == nil && v > 0 {
					q.StarCount = v
					found = true
				}
			}
		}
		if q.FWHM == 0 {
			if m := reFWHM.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil && v > 0 {
					q.FWHM = v
					q.FWHMUnit = "px"
					found = true
				}
			}
		}
		if q.Background == 0 {
			if m := reMedian.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil && v > 0 {
					q.Background = v
					found = true
				}
			}
		}
		if q.Noise == 0 {
			// Prefer bgnoise (background noise) over generic sigma.
			if m := reBgNoise.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil && v > 0 {
					q.Noise = v
					found = true
				}
			} else if m := reSigma.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil && v > 0 {
					q.Noise = v
					found = true
				}
			}
		}
	}

	if q.Background > 0 && q.Noise > 0 {
		q.SNR = q.Background / q.Noise
	}

	if !found {
		return q, fmt.Errorf("no recognisable Siril output")
	}
	return q, nil
}

// parsePlateSolveOutput extracts WCS coordinates from Siril plate solve output.
// Returns (result, true) only when both RA and Dec are confidently found.
func parsePlateSolveOutput(output string) (prefs.WCSResult, bool) {
	var r prefs.WCSResult
	raFound, decFound := false, false

	for _, line := range strings.Split(output, "\n") {
		if !raFound {
			if m := rePlateAlpha.FindStringSubmatch(line); m != nil {
				h, _ := strconv.ParseFloat(m[1], 64)
				min, _ := strconv.ParseFloat(m[2], 64)
				sec, _ := parseDecimal(m[3])
				r.RA = (h + min/60.0 + sec/3600.0) * 15.0
				raFound = true
			}
		}
		if !decFound {
			if m := rePlateDelta.FindStringSubmatch(line); m != nil {
				deg, _ := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
				min, _ := strconv.ParseFloat(m[2], 64)
				sec, _ := parseDecimal(m[3])
				sign := 1.0
				if deg < 0 {
					sign = -1.0
					deg = -deg
				}
				r.Dec = sign * (deg + min/60.0 + sec/3600.0)
				decFound = true
			}
		}
		if r.PixelScale == 0 {
			if m := rePlatePixScale.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					r.PixelScale = v
				}
			}
		}
		if r.Rotation == 0 {
			if m := rePlateRotation.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					r.Rotation = v
				}
			}
		}
	}

	return r, raFound && decFound
}

// parseDecimal handles both "." and "," as decimal separator.
func parseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
}
