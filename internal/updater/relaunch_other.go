//go:build !unix

package updater

import "errors"

// Relaunch isn't supported off Unix; no release is built for those platforms.
func Relaunch(path string) error {
	return errors.New("restart isn't supported on this platform")
}
