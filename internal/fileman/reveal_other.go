//go:build !linux && !darwin && !windows

package fileman

import (
	"os/exec"
	"path/filepath"
)

func configure(*exec.Cmd, command) {}

// Other Unix-likes (BSDs, …): no selection support, open the parent folder.
func revealCommands(path string) []command {
	return []command{{args: []string{"xdg-open", filepath.Dir(path)}}}
}

func openDirCommands(dir string) []command {
	return []command{{args: []string{"xdg-open", dir}}}
}
