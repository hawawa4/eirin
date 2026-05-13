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

// AnalyzeFrames runs Siril headless analysis (findstar + platesolve) on each NAS path
// sequentially. It emits "analysis:progress" events throughout and
// "library:updated" on completion so the frontend can refresh.
// Raw Siril output is written to $TMPDIR/eirin-siril-debug.txt for inspection.
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

	// Truncate debug log at the start of each analysis batch.
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

		// Log raw Siril output to debug file and Go console for inspection.
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
// and parses findstar + platesolve output. Always returns raw output bytes
// so the caller can write them to the debug log regardless of success/failure.
func analyzeSingleFrame(ctx context.Context, exe, nasPath string) (prefs.FrameQuality, prefs.WCSResult, bool, []byte, error) {
	escaped := strings.ReplaceAll(nasPath, `"`, `\"`)
	script := "requires 1.0.0\nload \"" + escaped + "\"\nfindstar\nplatesolve\n"

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

	// 120s to allow plate-solving catalog lookups.
	runCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, exe, "-s", tmp.Name())
	output, _ := cmd.CombinedOutput() // Siril may exit non-zero even on success

	if len(output) == 0 {
		return prefs.FrameQuality{}, prefs.WCSResult{}, false, output, fmt.Errorf("no output for %s", filepath.Base(nasPath))
	}
	q, err := parseFindstarOutput(string(output))
	if err != nil {
		return prefs.FrameQuality{}, prefs.WCSResult{}, false, output, fmt.Errorf("%s: %w", filepath.Base(nasPath), err)
	}
	wcs, wcsSolved := parsePlateSolveOutput(string(output))
	return q, wcs, wcsSolved, output, nil
}

// ── Output parsers ─────────────────────────────────────────────────────────

var (
	// "found N stars" or "N stars found/detected"
	reStarCount = regexp.MustCompile(`(?i)\b(\d+)\s+stars?\b`)

	// FWHM value optionally followed by arcsec marker or px
	// Handles: "FWHM: 2.34\"", "FWHM(x)=2.34 px", "Average FWHM: 2.34"
	reFWHM = regexp.MustCompile(`(?i)\bfwhm\b[^0-9\n]{0,25}?([0-9]+(?:[.,][0-9]+)?)\s*(")?`)

	reRoundness  = regexp.MustCompile(`(?i)\broundness\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
	reBackground = regexp.MustCompile(`(?i)\bbackground\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
	reNoise      = regexp.MustCompile(`(?i)\bnoise\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)
	reSNR        = regexp.MustCompile(`(?i)\bsnr\b[^0-9\n]{0,15}?([0-9]+(?:[.,][0-9]+)?)`)

	// Plate solve — RA/Dec in HMS/DMS (e.g. "05h 35m 17.3s", "-05° 23' 28"")
	rePlateRAHMS  = regexp.MustCompile(`(?i)\bra\b[^0-9\n]{0,10}([0-9]{1,3})[h: ][ ]?([0-9]{1,2})[m: ][ ]?([0-9]+(?:[.,][0-9]+)?)`)
	rePlateDecDMS = regexp.MustCompile(`(?i)\bdec\b[^0-9\n]{0,10}([+-]?[0-9]{1,3})[°d: ][ ]?([0-9]{1,2})['"m: ][ ]?([0-9]+(?:[.,][0-9]+)?)`)
	// Plate solve — RA/Dec in decimal degrees (fallback)
	rePlateRADeg  = regexp.MustCompile(`(?i)\bra\b\s*[=:]\s*([0-9]+(?:[.,][0-9]+)?)`)
	rePlateDecDeg = regexp.MustCompile(`(?i)\bdec\b\s*[=:]\s*([+-]?[0-9]+(?:[.,][0-9]+)?)`)
	// Pixel scale and rotation
	rePlatePixScale = regexp.MustCompile(`(?i)(?:pixel\s*scale|resolution)\s*[=:]\s*([0-9]+(?:[.,][0-9]+)?)\s*arcsec`)
	rePlateRotation = regexp.MustCompile(`(?i)(?:position\s*angle|image\s*(?:angle|orientation)|rotation|up\s*angle)\s*[=:]\s*([+-]?[0-9]+(?:[.,][0-9]+)?)`)
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

// parsePlateSolveOutput extracts WCS coordinates from Siril plate solve output.
// Returns (result, true) only when both RA and Dec are found.
// Tries HMS/DMS format first (more specific), falls back to decimal degrees.
func parsePlateSolveOutput(output string) (prefs.WCSResult, bool) {
	var r prefs.WCSResult
	raFound, decFound := false, false

	for _, line := range strings.Split(output, "\n") {
		if !raFound {
			if m := rePlateRAHMS.FindStringSubmatch(line); m != nil {
				h, _ := strconv.ParseFloat(m[1], 64)
				min, _ := strconv.ParseFloat(m[2], 64)
				sec, _ := parseDecimal(m[3])
				r.RA = (h + min/60.0 + sec/3600.0) * 15.0
				raFound = true
			} else if m := rePlateRADeg.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					r.RA = v
					raFound = true
				}
			}
		}
		if !decFound {
			if m := rePlateDecDMS.FindStringSubmatch(line); m != nil {
				degStr := strings.ReplaceAll(m[1], " ", "")
				deg, _ := strconv.ParseFloat(degStr, 64)
				min, _ := strconv.ParseFloat(m[2], 64)
				sec, _ := parseDecimal(m[3])
				sign := 1.0
				if deg < 0 {
					sign = -1.0
					deg = -deg
				}
				r.Dec = sign * (deg + min/60.0 + sec/3600.0)
				decFound = true
			} else if m := rePlateDecDeg.FindStringSubmatch(line); m != nil {
				if v, err := parseDecimal(m[1]); err == nil {
					r.Dec = v
					decFound = true
				}
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
