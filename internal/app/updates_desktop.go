//go:build !server

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/hawawa4/eirin/internal/updater"
)

const checkTimeout = 20 * time.Second

// updateSide tracks the in-app update: one install at a time, and the
// executable to relaunch once a new version is in place.
type updateSide struct {
	mu         sync.Mutex
	installing bool
	relaunch   string // set after a successful install
	pageURL    string // release page from the last check
}

// GetAppVersion returns the running version ("dev" for non-release builds).
func (a *App) GetAppVersion() string {
	return updater.Current
}

// CheckForUpdate asks GitHub for the latest release. It only runs when the
// user asks for it from Settings; nothing checks in the background.
func (a *App) CheckForUpdate() (UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	rel, err := updater.Client{}.Latest(ctx)
	if err != nil {
		return UpdateInfo{}, err
	}

	inst := updater.Detect()
	info := UpdateInfo{
		Current:   updater.Current,
		Latest:    rel.Version,
		Available: updater.Newer(rel.Version, updater.Current),
		Notes:     rel.Notes,
		PageURL:   rel.PageURL,
		Kind:      string(inst.Kind),
	}
	switch {
	case !updater.IsRelease(updater.Current):
		info.Reason = "This is a development build."
	case inst.Kind == updater.KindUnsupported:
		info.Reason = "In-app updates aren't available on this platform."
	case inst.Kind == updater.KindDeb:
		if why := updater.DebPrerequisites(); why != "" {
			info.Reason = "Installing the package needs " + why + "."
		} else {
			info.CanInstall = true
		}
	default:
		info.CanInstall = true
	}

	a.updates.mu.Lock()
	a.updates.pageURL = rel.PageURL
	a.updates.mu.Unlock()
	return info, nil
}

// InstallUpdate downloads and installs the latest release, reporting progress
// through "update:progress" events. The running app keeps going; RestartApp
// switches to the new version. For .deb installs this raises the system
// password prompt.
func (a *App) InstallUpdate() error {
	if err := a.requireWritable(); err != nil {
		return err
	}
	a.updates.mu.Lock()
	if a.updates.installing {
		a.updates.mu.Unlock()
		return errors.New("an update is already being installed")
	}
	a.updates.installing = true
	a.updates.mu.Unlock()
	defer func() {
		a.updates.mu.Lock()
		a.updates.installing = false
		a.updates.mu.Unlock()
	}()

	if !updater.IsRelease(updater.Current) {
		return errors.New("development builds can't be updated")
	}
	// Re-fetch rather than trust the earlier check: a newer release may have
	// been published since.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	client := updater.Client{}
	rel, err := client.Latest(ctx)
	if err != nil {
		return err
	}
	if !updater.Newer(rel.Version, updater.Current) {
		return errors.New("already running the latest version")
	}

	inst := updater.Detect()
	slog.Info("update: installing", "from", updater.Current, "to", rel.Version, "kind", inst.Kind)
	err = client.Install(ctx, rel, inst, func(p updater.Progress) {
		a.emitEvent("update:progress", p)
	})
	if err != nil {
		slog.Error("update: install", "err", err)
		return err
	}
	a.updates.mu.Lock()
	a.updates.relaunch = inst.Path
	a.updates.mu.Unlock()
	a.emitEvent("update:progress", updater.Progress{Phase: "done"})
	return nil
}

// RestartApp relaunches Eirin so an installed update takes effect.
func (a *App) RestartApp() error {
	if err := a.requireWritable(); err != nil {
		return err
	}
	a.updates.mu.Lock()
	path := a.updates.relaunch
	a.updates.mu.Unlock()
	if path == "" {
		return errors.New("no update has been installed")
	}
	a.imports.mu.Lock()
	importing := a.imports.running
	a.imports.mu.Unlock()
	if importing {
		return errors.New("an import is running; finish or cancel it before restarting")
	}
	if err := updater.Relaunch(path); err != nil {
		return err
	}
	if a.wails != nil {
		a.wails.Quit()
	}
	return nil
}

// OpenReleasePage opens the release page from the last update check in the
// default browser.
func (a *App) OpenReleasePage() error {
	a.updates.mu.Lock()
	url := a.updates.pageURL
	a.updates.mu.Unlock()
	if url == "" || a.wails == nil {
		return errors.New("no release page to open")
	}
	return a.wails.Browser.OpenURL(url)
}
