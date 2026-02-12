package watcher

import (
	"os"
	"strings"
	"sync"
)

var (
	wslOnce   sync.Once
	wslResult bool
)

// IsWSL returns true if the current environment is WSL (Windows Subsystem for Linux).
// The result is cached after the first call.
func IsWSL() bool {
	wslOnce.Do(func() {
		data, err := os.ReadFile("/proc/version")
		if err != nil {
			return
		}
		lower := strings.ToLower(string(data))
		wslResult = strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
	})
	return wslResult
}

// NeedsPollWatcher returns true if the given directory path requires poll-based
// watching. This is the case when running inside WSL and the path is a Windows
// mounted filesystem (under /mnt/).
func NeedsPollWatcher(path string) bool {
	return IsWSL() && strings.HasPrefix(path, "/mnt/")
}
