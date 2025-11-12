package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the default web browser to the specified URL.
// It automatically detects the operating system and uses the appropriate
// command to launch the browser (xdg-open on Linux, rundll32 on Windows,
// open on macOS). Returns an error if the platform is unsupported or if
// the browser fails to launch.
func OpenBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	return err
}

// TryOpenBrowser attempts to open the default web browser to the specified URL
// but silently ignores any errors. This is useful when browser access is optional
// and users can manually copy and paste the URL if automatic browser launching fails.
func TryOpenBrowser(url string) {
	// Silently ignore errors - user can still copy/paste the URL
	_ = OpenBrowser(url)
}
