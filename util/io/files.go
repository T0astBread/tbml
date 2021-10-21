package io

import (
	"errors"
	"io/fs"
	"os"
)

// FileModeURWGRWO is the bitmask for the Unix permission flags
// `u=rw,g=rw,o=`.
var FileModeURWGRWO os.FileMode = 0660

// FileModeURWXGRWXO is the bitmask for the Unix permission flags
// `u=rwx,g=rwx,o=`.
var FileModeURWXGRWXO os.FileMode = 0770

// DirExists returns if a directory exists at the given path, following symlinks.
func DirExists(name string) (bool, error) {
	stat, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return stat.IsDir(), nil
}

// FileExists returns if a file exists at the given path, following symlinks.
func FileExists(name string) (bool, error) {
	stat, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !stat.IsDir(), nil
}
