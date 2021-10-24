package io_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	uio "t0ast.cc/tbml/util/io"
)

type testDirFileInfo struct {
	IsDir bool
	Perms os.FileMode
}

func fromFileInfo(fileInfo os.FileInfo) testDirFileInfo {
	return testDirFileInfo{
		IsDir: fileInfo.IsDir(),
		Perms: fileInfo.Mode().Perm(),
	}
}

type testDir struct {
	baseInfo testDirFileInfo

	aContent []byte
	aInfo    testDirFileInfo

	bInfo testDirFileInfo

	cContent []byte
	cInfo    testDirFileInfo
}

func readTestDir(t *testing.T, path string) testDir {
	basePath := filepath.Join("testdata", path)
	assert.DirExists(t, basePath)
	baseInfo, err := os.Stat(basePath)
	assert.NoError(t, err)

	aPath := filepath.Join(basePath, "a.txt")
	aContent, err := os.ReadFile(aPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, aContent)
	aInfo, err := os.Stat(aPath)
	assert.NoError(t, err)

	bPath := filepath.Join(basePath, "b")
	assert.DirExists(t, bPath)
	bInfo, err := os.Stat(bPath)
	assert.NoError(t, err)

	cPath := filepath.Join(bPath, "c.json")
	cContent, err := os.ReadFile(cPath)
	assert.NoError(t, err)
	assert.NotEmpty(t, cContent)
	cInfo, err := os.Stat(cPath)
	assert.NoError(t, err)

	return testDir{
		baseInfo: fromFileInfo(baseInfo),

		aContent: aContent,
		aInfo:    fromFileInfo(aInfo),

		bInfo: fromFileInfo(bInfo),

		cContent: cContent,
		cInfo:    fromFileInfo(cInfo),
	}
}

func TestCopyDir(t *testing.T) {
	dir1Before := readTestDir(t, "dir-1")

	assert.NoError(t, os.RemoveAll("testdata/dir-2"))
	assert.NoError(t, uio.CopyDir("testdata/dir-1", "testdata/dir-2"))
	defer os.RemoveAll("testdata/dir-2")

	dir1After := readTestDir(t, "dir-1")
	assert.Equal(t, dir1Before, dir1After)

	dir2 := readTestDir(t, "dir-2")
	assert.Equal(t, dir1Before, dir2)
}
