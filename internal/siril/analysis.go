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

	"github.com/TaruDesigns/eirin/internal/store"
)

// AnalysisProgress describes the state of an in-progress frame analysis run.
type AnalysisProgress struct {
	Phase   string `json:"phase"`   // "analyzing" | "done" | "cancelled"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Current string `json:"current"` // current file basename
	Errors  int    `json:"errors"`
}

// AnalyzeSingleFrame runs findstar + platesolve + statistics for one frame.
// Returns quality metrics, WCS coordinates (if plate solve succeeded), raw output, and any error.
func AnalyzeSingleFrame(ctx context.Context, exe, nasPath string) (store.FrameQuality, store.WCSResult, bool, []byte, error) {
	escaped := strings.ReplaceAll(nasPath, `"`, `\"`)
	script := "requires 1.0.0\nload \"" + escaped + "\"\nfindstar\nplatesolve\n"

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

	if len(output) == 0 {
		return store.FrameQuality{}, store.WCSResult{}, false, output, fmt.Errorf("no output for %s", filepath.Base(nasPath))
	}
	q, err := ParseSirilOutput(string(output))
	if err != nil {
		return store.FrameQuality{}, store.WCSResult{}, false, output, fmt.Errorf("%s: %w", filepath.Base(nasPath), err)
	}
	wcs, wcsSolved := ParsePlateSolveOutput(string(output))
	return q, wcs, wcsSolved, output, nil
}

// ── Output parsers ─────────────────────────────────────────────────────────

var (
	// findstar: "Found 371 Gaussian profile stars in image, channel #0 (FWHM 3.383251)"
	reStarCount = regexp.MustCompile(`(?i)\bFound\s+(\d+)\s+\w+\s+\w+\s+stars?`)
	reFWHM      = regexp.MustCompile(`(?i)\bFWHM\s+([0-9]+(?:[.,][0-9]+)?)`)


	// platesolve: "Image center: alpha: 06 45 51.505, delta: -20 46 52.259"
	rePlateAlpha = regexp.MustCompile(`(?i)\balpha\s*:\s*([0-9]{1,3})\s+([0-9]{1,2})\s+([0-9]+(?:[.,][0-9]+)?)`)
	rePlateDelta = regexp.MustCompile(`(?i)\bdelta\s*:\s*([+-]?[0-9]{1,3})\s+([0-9]{1,2})\s+([0-9]+(?:[.,][0-9]+)?)`)
	// "Resolution:      3.672 arcsec/px"
	rePlatePixScale = regexp.MustCompile(`(?i)(?:pixel\s*scale|resolution)\s*[=:]\s*([0-9]+(?:[.,][0-9]+)?)\s*arcsec`)
	// "Up is +180.17 deg CounterclockWise wrt. N"
	rePlateRotation = regexp.MustCompile(`(?i)\bup\s+is\s+([+-]?[0-9]+(?:[.,][0-9]+)?)\s*deg`)
)

// ParseSirilOutput parses combined findstar + statistics output.
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
