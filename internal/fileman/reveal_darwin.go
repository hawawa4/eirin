//go:build darwin

package fileman

import "os/exec"

func configure(*exec.Cmd, command) {}

func revealCommands(path string) []command {
	return []command{{args: []string{"open", "-R", path}}}
}

func openDirCommands(dir string) []command {
	return []command{{args: []string{"open", dir}}}
}
