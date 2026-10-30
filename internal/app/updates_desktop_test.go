//go:build !server

package app

import (
	"testing"

	"github.com/hawawa4/eirin/internal/updater"
)

func TestRestartWithoutInstall(t *testing.T) {
	a := newTestApp(t)
	if err := a.RestartApp(); err == nil {
		t.Error("RestartApp should refuse when no update was installed")
	}
}

func TestRestartRefusedDuringImport(t *testing.T) {
	a := newTestApp(t)
	a.updates.relaunch = "/usr/bin/eirin"
	a.imports.running = true
	if err := a.RestartApp(); err == nil {
		t.Error("RestartApp should refuse while an import is running")
	}
}

func TestInstallRefusedForDevBuild(t *testing.T) {
	a := newTestApp(t)
	if updater.IsRelease(updater.Current) {
		t.Skip("test binary was built with a release version")
	}
	if err := a.InstallUpdate(); err == nil {
		t.Error("InstallUpdate should refuse a development build")
	}
}
