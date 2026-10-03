//go:build linux

package fileman

import (
	"net/url"
	"os/exec"
	"path/filepath"
)

func configure(*exec.Cmd, command) {}

// revealCommands asks the desktop's file manager to select path via the
// freedesktop FileManager1 D-Bus interface, falling back to opening the
// parent directory with xdg-open when no file manager implements it.
func revealCommands(path string) []command {
	return []command{
		{args: []string{
			"dbus-send", "--session", "--print-reply", "--reply-timeout=5000",
			"--dest=org.freedesktop.FileManager1",
			"--type=method_call",
			"/org/freedesktop/FileManager1",
			"org.freedesktop.FileManager1.ShowItems",
			"array:string:" + fileURI(path),
			"string:",
		}},
		{args: []string{"xdg-open", filepath.Dir(path)}},
	}
}

func openDirCommands(dir string) []command {
	return []command{{args: []string{"xdg-open", dir}}}
}

// fileURI builds a percent-escaped file:// URI for an absolute path.
func fileURI(path string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	return u.String()
}
