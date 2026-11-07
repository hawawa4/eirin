package siril

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/hawawa4/eirin/internal/store"
)

// AnalysisProgress describes the state of an in-progress frame analysis run.
type AnalysisProgress struct {
	Phase   string `json:"phase"`   // "analyzing" | "done" | "cancelled"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Current string `json:"current"` // current file basename
	Errors  int    `json:"errors"`
}

// AnalyzeSingleFrame runs findstar + stat + platesolve for one frame.
// hintRA/hintDec are decimal-degree center coordinates passed to platesolve as a search hint.
// Pass hintRA=0 and hintDec=0 for a blind solve (works for FITS with embedded WCS keywords;
// may fail for raster files that have no embedded coordinates).
// Returns quality metrics, WCS coordinates (if plate solve succeeded), raw output, and any error.
func AnalyzeSingleFrame(ctx context.Context, exe, nasPath string, hintRA, hintDec float64) (store.FrameQuality, store.WCSResult, bool, []byte, error) {
	escaped := strings.ReplaceAll(nasPath, `"`, `\"`)
	platesolveCmd := "platesolve"
	if hintRA != 0 || hintDec != 0 {
		platesolveCmd = fmt.Sprintf("platesolve %.6f %.6f", hintRA, hintDec)
	}
	// stat runs before platesolve: a failed solve (too few stars, no catalog)
	// aborts the script, and the quality metrics must survive that.
	script := "requires 1.0.0\nload \"" + escaped + "\"\nfindstar\nstat\n" + platesolveCmd + "\n"

	tmp, err := os.CreateTemp("", "eirin-siril-*.ssf")
	if err != nil {
		return store.FrameQuality{}, store.WCSResult{}, false, nil, fmt.Errorf("tmp script: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(script); err != nil {
		tmp.Close()
		return store.FrameQuality{}, store.WCSResult{}, false, nil, fmt.Errorf("write script: %w", err)
	}
	tmp.Close()

	// 120s covers plate-solve catalog lookup time.
	runCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(runCtx, exe, "-s", tmp.Name())
	output, _ := cmd.CombinedOutput() // Siril exits non-zero even on success

	// A timeout or cancellation says nothing about the file: retryable.
	if runCtx.Err() != nil {
		return store.FrameQuality{}, store.WCSResult{}, false, output, fmt.Errorf("%s: analysis interrupted or timed out: %w", filepath.Base(nasPath), runCtx.Err())
	}
	if len(output) == 0 {
		return store.FrameQuality{}, store.WCSResult{}, false, output, fmt.Errorf("no output for %s", filepath.Base(nasPath))
	}
	q, err := ParseSirilOutput(string(output))
	if err != nil {
		// If Siril never got as far as loading the file, it is Siril (or its
		// environment) that is broken, not the frame: stay retryable.
		if !strings.Contains(strings.ToLower(string(output)), "running command: load") {
			return store.FrameQuality{}, store.WCSResult{}, false, output, fmt.Errorf("%s: %w", filepath.Base(nasPath), err)
		}
		// Siril loaded (or tried to load) the file and found nothing to measure:
		// the file itself is the problem (damaged, truncated, unreadable), so
		// retrying won't help.
		return store.FrameQuality{}, store.WCSResult{}, false, output,
			&UnanalyzableError{File: filepath.Base(nasPath), Reason: FailureReason(string(output)), Cause: err}
	}
	wcs, wcsSolved := ParsePlateSolveOutput(string(output))
	return q, wcs, wcsSolved, output, nil
}

// UnanalyzableError means Siril ran but could not extract anything from the
// frame, so analysing it again will fail the same way. Reason is a short,
// fixed, path-free sentence that is safe to store and show.
type UnanalyzableError struct {
	File   string
	Reason string
	Cause  error
}

func (e *UnanalyzableError) Error() string { return fmt.Sprintf("%s: %s (%v)", e.File, e.Reason, e.Cause) }
func (e *UnanalyzableError) Unwrap() error { return e.Cause }

// FailureReason summarises why Siril produced no usable output. The result is
// deliberately one of a few fixed strings: raw Siril output contains file paths.
func FailureReason(output string) string {
	l := strings.ToLower(output)
	switch {
	case strings.Contains(l, "fitsio error") || strings.Contains(l, "fits error"):
		return "Siril could not read the file's pixel data (damaged or truncated)"
	case strings.Contains(l, "file not found") || (strings.Contains(l, "opening") && strings.Contains(l, "failed")):
		return "Siril could not open the file"
	default:
		return "Siril produced no usable output for this file"
	}
}

// ── Output parsers ─────────────────────────────────────────────────────────

var (
	// findstar: "Found 371 Gaussian profile stars in image, channel #0 (FWHM 3.383251)"
	reStarCount = regexp.MustCompile(`(?i)\bFound\s+(\d+)\s+\w+\s+\w+\s+stars?`)
	reFWHM      = regexp.MustCompile(`(?i)\bFWHM\s+([0-9]+(?:[.,][0-9]+)?)`)

	// stat: "B&W layer: Mean: 4041.4, Median: 4063.0, Sigma: 515.5, Min: 0.0, Max: 65535.0, bgnoise: 446.1"
	reMedian = regexp.MustCompile(`(?i)\bmedian\s*:\s*([0-9]+(?:[.,][0-9]+)?)`)
	reBgNoise = regexp.MustCompile(`(?i)\bbgnoise\s*:\s*([0-9]+(?:[.,][0-9]+)?)`)
	reSigma   = regexp.MustCompile(`(?i)\bsigma\s*:\s*([0-9]+(?:[.,][0-9]+)?)`)

	// platesolve: "Image center: alpha: 06 45 51.505, delta: -20 46 52.259"
	rePlateAlpha = regexp.MustCompile(`(?i)\balpha\s*:\s*([0-9]{1,3})\s+([0-9]{1,2})\s+([0-9]+(?:[.,][0-9]+)?)`)
	rePlateDelta = regexp.MustCompile(`(?i)\bdelta\s*:\s*([+-]?[0-9]{1,3})\s+([0-9]{1,2})\s+([0-9]+(?:[.,][0-9]+)?)`)
	// "Resolution:      3.672 arcsec/px"
	rePlatePixScale = regexp.MustCompile(`(?i)(?:pixel\s*scale|resolution)\s*[=:]\s*([0-9]+(?:[.,][0-9]+)?)\s*arcsec`)
	// "Up is +180.17 deg CounterclockWise wrt. N"
	rePlateRotation = regexp.MustCompile(`(?i)\bup\s+is\s+([+-]?[0-9]+(?:[.,][0-9]+)?)\s*deg`)
)

// ParseSirilOutput parses combined findstar + stat output.
// SNR is derived as background/noise when both are available.
func ParseSirilOutput(output string) (store.FrameQuality, error) {
	var q store.FrameQuality
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
				if v, err := ParseDecimal(m[1]); err == nil && v > 0 {
					q.FWHM = v
					q.FWHMUnit = "px"
					found = true
				}
			}
		}
		if q.Background == 0 {
			if m := reMedian.FindStringSubmatch(line); m != nil {
				if v, err := ParseDecimal(m[1]); err == nil {
					q.Background = v
					found = true
				}
			}
		}
		// bgnoise is preferred over sigma; only fall back to sigma if bgnoise wasn't seen.
		if m := reBgNoise.FindStringSubmatch(line); m != nil {
			if v, err := ParseDecimal(m[1]); err == nil {
				q.Noise = v
				found = true
			}
		} else if q.Noise == 0 {
			if m := reSigma.FindStringSubmatch(line); m != nil {
				if v, err := ParseDecimal(m[1]); err == nil {
					q.Noise = v
					found = true
				}
			}
		}
	}

	if q.Noise != 0 {
		q.SNR = q.Background / q.Noise
	}

	if !found {
		return q, fmt.Errorf("no recognisable Siril output")
	}
	return q, nil
}

// ParsePlateSolveOutput extracts WCS coordinates from Siril plate solve output.
// Returns (result, true) only when both RA and Dec are confidently found.
func ParsePlateSolveOutput(output string) (store.WCSResult, bool) {
	var r store.WCSResult
	raFound, decFound := false, false

	for _, line := range strings.Split(output, "\n") {
		if !raFound {
			if m := rePlateAlpha.FindStringSubmatch(line); m != nil {
				h, _ := strconv.ParseFloat(m[1], 64)
				min, _ := strconv.ParseFloat(m[2], 64)
				sec, _ := ParseDecimal(m[3])
				r.RA = (h + min/60.0 + sec/3600.0) * 15.0
				raFound = true
			}
		}
		if !decFound {
			if m := rePlateDelta.FindStringSubmatch(line); m != nil {
				deg, _ := strconv.ParseFloat(strings.TrimSpace(m[1]), 64)
				min, _ := strconv.ParseFloat(m[2], 64)
				sec, _ := ParseDecimal(m[3])
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
				if v, err := ParseDecimal(m[1]); err == nil {
					r.PixelScale = v
				}
			}
		}
		if r.Rotation == 0 {
			if m := rePlateRotation.FindStringSubmatch(line); m != nil {
				if v, err := ParseDecimal(m[1]); err == nil {
					r.Rotation = v
				}
			}
		}
	}

	return r, raFound && decFound
}

// ParseDecimal handles both "." and "," as decimal separator.
func ParseDecimal(s string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
}
