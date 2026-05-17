package siril

import "testing"

func TestExecutableDefault(t *testing.T) {
	if got := Executable(""); got != "siril" {
		t.Errorf("Executable(\"\") = %q, want siril", got)
	}
}

func TestExecutableConfigured(t *testing.T) {
	if got := Executable("/opt/siril/bin/siril"); got != "/opt/siril/bin/siril" {
		t.Errorf("Executable() = %q, want /opt/siril/bin/siril", got)
	}
}

func TestExecutableConfiguredOverridesDefault(t *testing.T) {
	got := Executable("/custom/siril")
	if got == "siril" {
		t.Error("configured path should override default 'siril'")
	}
	if got != "/custom/siril" {
		t.Errorf("Executable() = %q, want /custom/siril", got)
	}
}
