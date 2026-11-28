package browser

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("unsupported platform for browser open: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// OpenFile opens a local HTML browser handoff and catches immediate opener
// errors. On Linux an opener can stay attached to a healthy browser.
func OpenFile(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// An editor may own .html even when a browser owns HTTP.
		script := `ObjC.import("AppKit"); ObjC.unwrap($.NSWorkspace.sharedWorkspace.URLForApplicationToOpenURL($.NSURL.URLWithString("http://127.0.0.1")).path)`
		app, err := exec.Command("/usr/bin/osascript", "-l", "JavaScript", "-e", script).Output()
		if err != nil || strings.TrimSpace(string(app)) == "" {
			return fmt.Errorf("could not locate the default HTTP browser")
		}
		cmd = exec.Command("/usr/bin/open", "-a", strings.TrimSpace(string(app)), path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	default:
		return fmt.Errorf("unsupported browser platform %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		return nil
	}
}
