package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	sirilpkg "github.com/TaruDesigns/eirin/internal/siril"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// AnalysisProgress is emitted as "analysis:progress" during frame analysis.
// Re-exported from the siril package for Wails binding compatibility.
type AnalysisProgress = sirilpkg.AnalysisProgress

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

		quality, wcs, wcsSolved, rawOutput, err := sirilpkg.AnalyzeSingleFrame(ctx, exe, path)

		if df, ferr := os.OpenFile(debugPath, os.O_APPEND|os.O_WRONLY, 0644); ferr == nil {
			fmt.Fprintf(df, "\n--- %s ---\n%s\n", filepath.Base(path), string(rawOutput))
			df.Close()
		}
		slog.Info("siril output", "file", filepath.Base(path), "output", string(rawOutput))

		if err != nil {
			slog.Warn("analysis", "err", err)
			errCount++
		} else {
			if uerr := a.store.UpdateFrameQuality(path, quality); uerr != nil {
				slog.Warn("analysis: db quality update", "err", uerr)
				errCount++
			}
			if wcsSolved {
				if uerr := a.store.UpdateWCS(path, wcs); uerr != nil {
					slog.Warn("analysis: db wcs update", "err", uerr)
				}
			}
		}
	}

	a.emitAnalysisProgress("done", total, total, "", errCount)
	a.wails.Event.EmitEvent(&application.CustomEvent{Name: "library:updated"})
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
	a.wails.Event.EmitEvent(&application.CustomEvent{Name: "analysis:progress", Data: AnalysisProgress{
		Phase:   phase,
		Total:   total,
		Done:    done,
		Current: current,
		Errors:  errors,
	}})
}
