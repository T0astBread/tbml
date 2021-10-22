package internal

import (
	"path/filepath"
	"time"
)

const genericErrorExitCode = 1

type Configuration struct {
	ProfilePath string
	Profiles    []ProfileConfiguration
}

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

func getInstanceDir(config Configuration, instance ProfileInstance) string {
	return filepath.Join(config.ProfilePath, instance.InstanceLabel)
}
