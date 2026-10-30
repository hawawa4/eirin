package updater

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// binaryName is the executable inside the release .zip.
const binaryName = "eirin"

// maxBinarySize bounds extraction so a corrupt archive can't fill the disk.
const maxBinarySize = 1 << 30

// extractBinary writes the executable inside zipPath to dest.
func extractBinary(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || path.Base(zf.Name) != binaryName {
			continue
		}
		src, err := zf.Open()
		if err != nil {
			return err
		}
		defer func() { _ = src.Close() }()
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		n, err := io.Copy(out, io.LimitReader(src, maxBinarySize+1))
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err == nil && n > maxBinarySize {
			err = errors.New("archive entry is too large")
		}
		return err
	}
	return fmt.Errorf("no %q executable in the downloaded archive", binaryName)
}

// replaceFile atomically swaps newPath in for target. Both must be on the same
// filesystem; the old file stays valid for the running process, which keeps
// its own inode.
func replaceFile(newPath, target string) error {
	mode := os.FileMode(0o755)
	if st, err := os.Stat(target); err == nil {
		mode = st.Mode().Perm() | 0o111
	}
	if err := os.Chmod(newPath, mode); err != nil {
		return err
	}
	return os.Rename(newPath, target)
}

// tempBeside creates an empty temp file in the same directory as target, so a
// later rename can't cross filesystems. It fails when that directory isn't
// writable, which is the "can't self-update here" case.
func tempBeside(target string) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(target), ".eirin-update-*")
	if err != nil {
		return "", fmt.Errorf("can't write next to %s: %w", target, err)
	}
	name := f.Name()
	_ = f.Close()
	return name, nil
}

// debInstallCmd runs apt through pkexec: a GUI app has no terminal for sudo,
// while polkit shows the desktop's own password dialog. pkexec clears the
// environment, so apt is told not to ask questions.
func debInstallCmd(ctx context.Context, debPath string) *exec.Cmd {
	return exec.CommandContext(ctx, "pkexec", "env", "DEBIAN_FRONTEND=noninteractive",
		"apt-get", "install", "-y", debPath)
}

// DebPrerequisites reports why a .deb update can't be run from here, or "".
func DebPrerequisites() string {
	for _, tool := range []string{"pkexec", "apt-get"} {
		if _, err := exec.LookPath(tool); err != nil {
			return tool + " isn't installed"
		}
	}
	return ""
}

// ErrAuthCancelled is returned when the user dismissed the password dialog.
var ErrAuthCancelled = errors.New("the password prompt was cancelled")

// runDebInstall is a variable so tests can avoid invoking pkexec.
var runDebInstall = func(ctx context.Context, debPath string) error {
	out, err := debInstallCmd(ctx, debPath).CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case 126: // pkexec: dialog dismissed
			return ErrAuthCancelled
		case 127: // pkexec: not authorized
			return errors.New("not authorised to install packages")
		}
	}
	if err != nil {
		return fmt.Errorf("apt-get failed: %w: %s", err, tail(string(out), 400))
	}
	return nil
}

// tail returns at most n trailing bytes of s, trimmed.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "…" + s[len(s)-n:]
	}
	return s
}
