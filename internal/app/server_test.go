package app

import "testing"

func TestGetEnv(t *testing.T) {
	t.Setenv("EIRIN_TEST_GETENV_KEY", "hello")
	if got := getEnv("EIRIN_TEST_GETENV_KEY"); got != "hello" {
		t.Errorf("getEnv = %q, want hello", got)
	}
	if got := getEnv("EIRIN_TEST_NONEXISTENT_KEY_XYZ"); got != "" {
		t.Errorf("getEnv (missing key) = %q, want empty string", got)
	}
}

func TestServerPortDefault(t *testing.T) {
	t.Setenv("EIRIN_PORT", "")
	if got := serverPort(); got != defaultServerPort {
		t.Errorf("serverPort() = %d, want default %d", got, defaultServerPort)
	}
}

func TestServerPortEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "9090")
	if got := serverPort(); got != 9090 {
		t.Errorf("serverPort() = %d, want 9090", got)
	}
}

func TestServerPortInvalidEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "notanumber")
	if got := serverPort(); got != defaultServerPort {
		t.Errorf("serverPort() with non-numeric value = %d, want default %d", got, defaultServerPort)
	}
}

func TestServerPortOutOfRangeEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "99999")
	if got := serverPort(); got != defaultServerPort {
		t.Errorf("serverPort() with out-of-range value = %d, want default %d", got, defaultServerPort)
	}
}

func TestServerPortZeroEnvVar(t *testing.T) {
	t.Setenv("EIRIN_PORT", "0")
	if got := serverPort(); got != defaultServerPort {
		t.Errorf("serverPort() with 0 = %d, want default %d", got, defaultServerPort)
	}
}
