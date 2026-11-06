//go:build !server

package app

// publicPath is how the build's frontend names a library file in requests:
// the desktop uses real paths.
func publicPath(_ *App, real string) string { return real }
