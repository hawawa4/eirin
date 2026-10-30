//go:build unix

package updater

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// Relaunch starts path in a new session once this process has exited, so the
// new version doesn't race the old one for the database or the API port. The
// caller quits the app right after.
func Relaunch(path string) error {
	cmd := exec.Command("/bin/sh", "-c",
		`while kill -0 "$1" 2>/dev/null; do sleep 0.2; done; exec "$0"`,
		path, strconv.Itoa(os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
