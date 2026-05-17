package app

import (
	"testing"

	"github.com/TaruDesigns/eirin/internal/server"
)

func TestGetEnv(t *testing.T) {
	t.Setenv("EIRIN_TEST_GETENV_KEY", "hello")
	if got := server.GetEnv("EIRIN_TEST_GETENV_KEY"); got != "hello" {
		t.Errorf("GetEnv = %q, want hello", got)
	}
	if got := server.GetEnv("EIRIN_TEST_NONEXISTENT_KEY_XYZ"); got != "" {
		t.Errorf("GetEnv (missing key) = %q, want empty string", got)
	}
}

func TestServerPortDefault(t *testing.T) {
	t.Setenv("EIRIN_PORT", "")
	if got := server.Port(); got != server.DefaultPort {
		t.Errorf("Port() = %d, want default %d", got, server.DefaultPort)
	}
}

func TestServerPortEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "9090")
	if got := server.Port(); got != 9090 {
		t.Errorf("Port() = %d, want 9090", got)
	}
}

func TestServerPortInvalidEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "notanumber")
	if got := server.Port(); got != server.DefaultPort {
		t.Errorf("Port() with non-numeric value = %d, want default %d", got, server.DefaultPort)
	}
}

func TestServerPortOutOfRangeEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "99999")
	if got := server.Port(); got != server.DefaultPort {
		t.Errorf("Port() with out-of-range value = %d, want default %d", got, server.DefaultPort)
	}
}

func TestServerPortZeroEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "0")
	if got := server.Port(); got != server.DefaultPort {
		t.Errorf("Port() with 0 = %d, want default %d", got, server.DefaultPort)
	}
}
