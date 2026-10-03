// Package fileman reveals files and opens folders in the operating system's
// file manager (Nautilus/Dolphin/… on Linux, Finder on macOS, Explorer on
// Windows). Command construction is kept in pure per-OS functions so it can
// be unit-tested without spawning anything.
package fileman

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// command is a single process invocation. ignoreExit marks commands whose
// exit status carries no meaning (Windows explorer returns 1 on success).
type command struct {
	args       []string
	ignoreExit bool
}

// Reveal opens the OS file manager with path selected (or, where selection
// isn't supported, opens the containing folder). The file manager is started
// in the background; Reveal does not wait for it to exit.
func Reveal(path string) error {
	abs, _, err := resolve(path)
	if err != nil {
		return err
	}
	return runFirst(revealCommands(abs))
}

// OpenDir opens path in the OS file manager. If path is a file, its parent
// directory is opened instead.
func OpenDir(path string) error {
	abs, info, err := resolve(path)
	if err != nil {
		return err
	}
	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	return runFirst(openDirCommands(dir))
}

// resolve validates that path exists and returns its absolute form.
func resolve(path string) (string, os.FileInfo, error) {
	if path == "" {
		return "", nil, errors.New("no path given")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("resolving %q: %w", path, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("path does not exist: %s", abs)
		}
		return "", nil, fmt.Errorf("cannot access %s: %w", abs, err)
	}
	return abs, info, nil
}

// runFirst tries each command in order and stops at the first that starts
// (and, for commands that report something meaningful, exits successfully).
//
// Most commands are fire-and-forget: they are started and reaped in the
// background. A command with a fallback after it (e.g. the D-Bus call on
// Linux) is waited on so a failure can trigger the fallback — those calls
// return immediately once the file manager has been asked to show the item.
func runFirst(cmds []command) error {
	if len(cmds) == 0 {
		return errors.New("revealing files is not supported on this platform")
	}
	var lastErr error
	for i, c := range cmds {
		hasFallback := i < len(cmds)-1
		cmd := exec.Command(c.args[0], c.args[1:]...)
		configure(cmd, c)
		if hasFallback && !c.ignoreExit {
			if err := cmd.Run(); err != nil {
				slog.Debug("fileman: command failed, trying fallback", "cmd", c.args[0], "err", err)
				lastErr = err
				continue
			}
			return nil
		}
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
	return fmt.Errorf("opening file manager: %w", lastErr)
}
