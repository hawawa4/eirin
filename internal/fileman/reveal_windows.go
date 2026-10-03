//go:build windows

package fileman

import (
	"os/exec"
	"strings"
	"syscall"
)

const selectFlag = "/select,"

// explorer.exe returns exit code 1 even when it succeeds, so its exit status
// is ignored.
func revealCommands(path string) []command {
	return []command{{args: []string{"explorer", selectFlag + path}, ignoreExit: true}}
}

func openDirCommands(dir string) []command {
	return []command{{args: []string{"explorer", dir}, ignoreExit: true}}
}

// configure passes explorer's command line verbatim: its /select, switch
// only accepts the path quoted on its own (`/select,"C:\a b\c.fit"`), which
// Go's default argument escaping can't produce.
func configure(cmd *exec.Cmd, c command) {
	if len(c.args) != 2 || c.args[0] != "explorer" {
		return
	}
	line := `explorer "` + c.args[1] + `"`
	if p, ok := strings.CutPrefix(c.args[1], selectFlag); ok {
		line = `explorer ` + selectFlag + `"` + p + `"`
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
}
