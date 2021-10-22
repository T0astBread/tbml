package main

import (
	"os"
	"path/filepath"
	"time"

	uerror "t0ast.cc/tbml/util/error"
)

const genericErrorExitCode = 1

type ProfileConfiguration struct {
	ExtensionFiles []string
	Label          string
	UserChromeFile *string
	UserJSFile     *string
}

type ProfileInstance struct {
	Created       time.Time
	InstanceLabel string
	LastUsed      time.Time
	ProfileLabel  string
	UsageLabel    *string
	UsagePID      *int
}

func getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", uerror.WithStackTrace(err)
	}
	return filepath.Join(home, ".cache", "tbml"), nil
}

func getInstanceDir(instance ProfileInstance) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", uerror.WithStackTrace(err)
	}
	return filepath.Join(cacheDir, instance.InstanceLabel), nil
}
