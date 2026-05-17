package server

import (
	"os"
	"strconv"
)

const DefaultPort = 7070

// GetEnv returns the value of an environment variable.
func GetEnv(key string) string { return os.Getenv(key) }

// Port returns the HTTP server port from the EIRIN_PORT env var, or the default.
func Port() int {
	if v := GetEnv("EIRIN_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n < 65536 {
			return n
		}
	}
	return DefaultPort
}
