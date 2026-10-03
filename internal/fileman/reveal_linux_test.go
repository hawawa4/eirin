//go:build linux

package fileman

import "testing"

func TestRevealCommandsLinux(t *testing.T) {
	cmds := revealCommands("/mnt/nas/M31 Ha/Light_#1.fit")
	if len(cmds) != 2 {
		t.Fatalf("got %d commands, want 2", len(cmds))
	}
	dbus := cmds[0].args
	if dbus[0] != "dbus-send" {
		t.Fatalf("first command = %q, want dbus-send", dbus[0])
	}
	wantURI := "array:string:file:///mnt/nas/M31%20Ha/Light_%231.fit"
	if dbus[len(dbus)-2] != wantURI {
		t.Errorf("uri arg = %q, want %q", dbus[len(dbus)-2], wantURI)
	}
	if dbus[len(dbus)-1] != "string:" {
		t.Errorf("startup id arg = %q, want empty string:", dbus[len(dbus)-1])
	}
	fb := cmds[1].args
	if fb[0] != "xdg-open" || fb[1] != "/mnt/nas/M31 Ha" {
		t.Errorf("fallback = %v, want xdg-open on parent dir", fb)
	}
}

func TestOpenDirCommandsLinux(t *testing.T) {
	cmds := openDirCommands("/mnt/nas")
	if len(cmds) != 1 || cmds[0].args[0] != "xdg-open" || cmds[0].args[1] != "/mnt/nas" {
		t.Fatalf("openDirCommands = %+v", cmds)
	}
}
