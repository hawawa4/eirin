package server

import "testing"

func TestGetEnv(t *testing.T) {
	t.Setenv("EIRIN_TEST_GETENV_KEY", "hello")
	if got := GetEnv("EIRIN_TEST_GETENV_KEY"); got != "hello" {
		t.Errorf("GetEnv = %q, want hello", got)
	}
	if got := GetEnv("EIRIN_TEST_NONEXISTENT_KEY_XYZ"); got != "" {
		t.Errorf("GetEnv (missing key) = %q, want empty string", got)
	}
}

func TestPortDefault(t *testing.T) {
	t.Setenv("EIRIN_PORT", "")
	if got := Port(); got != DefaultPort {
		t.Errorf("Port() = %d, want default %d", got, DefaultPort)
	}
}

func TestPortEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "9090")
	if got := Port(); got != 9090 {
		t.Errorf("Port() = %d, want 9090", got)
	}
}

func TestPortInvalidEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "notanumber")
	if got := Port(); got != DefaultPort {
		t.Errorf("Port() with non-numeric value = %d, want default %d", got, DefaultPort)
	}
}

func TestPortOutOfRangeEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "99999")
	if got := Port(); got != DefaultPort {
		t.Errorf("Port() with out-of-range value = %d, want default %d", got, DefaultPort)
	}
}

func TestPortZeroEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "0")
	if got := Port(); got != DefaultPort {
		t.Errorf("Port() with 0 = %d, want default %d", got, DefaultPort)
	}
}
