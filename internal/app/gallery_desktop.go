//go:build !server

package app

// publicToReal maps a path from the frontend to a library path. The desktop
// frontend already uses real paths.
func (a *App) publicToReal(path string) (string, bool) { return path, true }
