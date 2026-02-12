package watcher

import (
	"testing"
)

func TestNeedsPollWatcher_NonMntPath(t *testing.T) {
	// Even if IsWSL() returns true, a non-/mnt/ path should not need poll watcher.
	// On non-WSL systems, NeedsPollWatcher always returns false regardless of path.
	if NeedsPollWatcher("/home/user/Documents") {
		t.Error("NeedsPollWatcher should return false for non-/mnt/ path")
	}
}

func TestNeedsPollWatcher_MntPathOnCurrentEnv(t *testing.T) {
	result := NeedsPollWatcher("/mnt/c/Users/test")
	if IsWSL() {
		if !result {
			t.Error("NeedsPollWatcher should return true for /mnt/ path on WSL")
		}
	} else {
		if result {
			t.Error("NeedsPollWatcher should return false for /mnt/ path on non-WSL")
		}
	}
}

func TestIsWSL_DoesNotPanic(t *testing.T) {
	// Ensure IsWSL() does not panic regardless of environment.
	_ = IsWSL()
}
