package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser opens the default browser to the specified URL
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

// TryOpenBrowser attempts to open the browser but doesn't fail if it can't
func TryOpenBrowser(url string) {
	// Silently ignore errors - user can still copy/paste the URL
	_ = OpenBrowser(url)
}
