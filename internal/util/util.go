package util

import "os"

// EnsureDirs creates the directory at path (and all parents) with 0700 permissions.
// If path is empty, it does nothing and returns nil.
func EnsureDirs(path string) error {
	if path == "" {
		return nil
	}
	return os.MkdirAll(path, 0700)
}
