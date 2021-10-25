package io

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// CopyFile copies the `src` file to `dst`.
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	fileInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// CopyDir copies all files in the `src` directroy into `dst`,
// preserving permissions.
func CopyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dstPath := strings.TrimPrefix(path, src)
		dstPath = strings.TrimPrefix(dstPath, "/")
		dstPath = filepath.Join(dst, dstPath)
		fileInfo, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := os.MkdirAll(dstPath, fileInfo.Mode()); err != nil {
				return err
			}
		} else if err := copyDirFile(path, dstPath, fileInfo); err != nil {
			return err
		}
		return nil
	})
}

func copyDirFile(path, dst string, fileInfo fs.FileInfo) error {
	srcFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}
