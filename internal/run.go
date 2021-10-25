package internal

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	uerror "t0ast.cc/tbml/util/error"
	uio "t0ast.cc/tbml/util/io"
	ustring "t0ast.cc/tbml/util/string"
)

const tblFirejailProfileFileName = "torbrowser-launcher.profile"

const relativeProfilePath = ".local/share/torbrowser/tbb/x86_64/tor-browser_en-US/Browser/TorBrowser/Data/Browser/profile.default"

//go:embed torbrowser-launcher.profile
var tblFirejailProfile []byte

//go:embed torbrowser-launcher-default-settings.json
var tblDefaultSettings []byte

func StartInstance(ctx context.Context, config Configuration, profile ProfileConfiguration, instance ProfileInstance, allInstances []ProfileInstance, configDir string, debugShell bool) (exitCode uint, err error) {
	instanceDir := getInstanceDir(config, instance)

	cleanUpInstanceData, err := writeInstanceData(config, profile, instance)
	if err != nil {
		return genericErrorExitCode, uerror.WithStackTrace(err)
	}
	defer cleanUpInstanceData()

	if err := ensureFiles(profile, configDir, instanceDir); err != nil {
		return genericErrorExitCode, uerror.WithStackTrace(err)
	}

	if err := ensureExtensions(config, profile, instance.InstanceLabel, configDir, instanceDir); err != nil {
		return genericErrorExitCode, uerror.WithStackTrace(err)
	}

	if err := writePortSettings(instanceDir, allInstances); err != nil {
		return genericErrorExitCode, uerror.WithStackTrace(err)
	}

	cleanUpBindMounts, err := setUpBindMounts(instanceDir)
	if err != nil {
		return genericErrorExitCode, uerror.WithStackTrace(err)
	}
	defer cleanUpBindMounts()

	return runFirejail(ctx, instanceDir, debugShell)
}

func writeInstanceData(config Configuration, profile ProfileConfiguration, instance ProfileInstance) (cleanup func() error, err error) {
	instanceDir := getInstanceDir(config, instance)

	instanceDataPath := filepath.Join(instanceDir, "profile-instance.json")

	instanceExists, err := uio.FileExists(instanceDataPath)
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	if !instanceExists {
		instance.Created = time.Now()
		if err := os.MkdirAll(instanceDir, uio.FileModeURWXGRWXO); err != nil {
			return nil, uerror.WithStackTrace(err)
		}
	}

	pid := os.Getpid()
	instance.LastUsed = time.Now()
	instance.UsagePID = &pid

	marshalData := func(instance ProfileInstance) error {
		instanceDataBytes, err := json.Marshal(instance)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if err := os.WriteFile(instanceDataPath, instanceDataBytes, uio.FileModeURWGRWO); err != nil {
			return uerror.WithStackTrace(err)
		}
		return nil
	}

	if err := marshalData(instance); err != nil {
		return nil, err
	}

	return func() error {
		instanceDataBytes, err := os.ReadFile(instanceDataPath)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		instance = ProfileInstance{}
		json.Unmarshal(instanceDataBytes, &instance)

		instance.LastUsed = time.Now()
		instance.UsageLabel = nil
		instance.UsagePID = nil
		return marshalData(instance)
	}, nil
}

func ensureFiles(profile ProfileConfiguration, configDir string, instanceDir string) error {
	tblSettingsPath := filepath.Join(instanceDir, ".config/torbrowser/settings.json")
	if err := writeIfNotExists(tblSettingsPath, tblDefaultSettings); err != nil {
		return uerror.WithStackTrace(err)
	}

	tblFirejailProfilePath := filepath.Join(instanceDir, tblFirejailProfileFileName)
	if err := ensureExists(tblFirejailProfilePath, tblFirejailProfile); err != nil {
		return uerror.WithStackTrace(err)
	}

	profileDir := filepath.Join(instanceDir, relativeProfilePath)

	userChromePath := filepath.Join(profileDir, "chrome/userChrome.css")
	if profile.UserChromeFile == nil {
		userChromeExists, err := uio.FileExists(userChromePath)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if userChromeExists {
			if err := os.Remove(userChromePath); err != nil {
				return uerror.WithStackTrace(err)
			}
		}
	} else {
		ensureExistsFrom(userChromePath, filepath.Join(configDir, *profile.UserChromeFile))
	}

	userJSPath := filepath.Join(profileDir, "user.js")
	if profile.UserJSFile == nil {
		userJSExists, err := uio.FileExists(userJSPath)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if userJSExists {
			if err := os.Remove(userJSPath); err != nil {
				return uerror.WithStackTrace(err)
			}
		}
	} else {
		ensureExistsFrom(userJSPath, filepath.Join(configDir, *profile.UserJSFile))
	}

	return nil
}

func ensureExtensions(config Configuration, profile ProfileConfiguration, instanceLabel, configDir, instanceDir string) error {
	instance, err := GetProfileInstance(config, instanceLabel)
	if err != nil {
		return uerror.WithStackTrace(err)
	}

	wantedExtensions := make(map[string]bool)
	extensionPathByID := make(map[string]string)
	for _, extensionID := range instance.InstalledExtensions {
		wantedExtensions[extensionID] = false
	}
	for _, extensionFilePath := range profile.ExtensionFiles {
		extensionID := strings.TrimSuffix(filepath.Base(extensionFilePath), ".xpi")
		wantedExtensions[extensionID] = true
		extensionPathByID[extensionID] = extensionFilePath
	}
	for extensionID, wanted := range wantedExtensions {
		extensionPathInProfile := filepath.Join(instanceDir, relativeProfilePath, "extensions", fmt.Sprint(extensionID, ".xpi"))
		if wanted {
			extensionSrcPath := extensionPathByID[extensionID]
			if !filepath.IsAbs(extensionSrcPath) {
				extensionSrcPath = filepath.Join(configDir, extensionSrcPath)
			}
			if err := ensureExistsFrom(extensionPathInProfile, extensionSrcPath); err != nil {
				return uerror.WithStackTrace(err)
			}
			instance.InstalledExtensions = includeExtension(instance.InstalledExtensions, extensionID)
		} else {
			if err := os.Remove(extensionPathInProfile); err != nil {
				return uerror.StackTracef("Couldn't delete installed extension %s: %w", extensionID, err)
			}
			instance.InstalledExtensions = excludeExtension(instance.InstalledExtensions, extensionID)
		}
	}

	instanceDataBytes, err := json.Marshal(instance)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	instanceDataPath := filepath.Join(instanceDir, "profile-instance.json")
	if err := os.WriteFile(instanceDataPath, instanceDataBytes, uio.FileModeURWGRWO); err != nil {
		return uerror.WithStackTrace(err)
	}

	return nil
}

func includeExtension(extensionList []string, extensionID string) []string {
	for _, idInList := range extensionList {
		if idInList == extensionID {
			return extensionList
		}
	}
	return append(extensionList, extensionID)
}

func excludeExtension(extensionList []string, extensionID string) []string {
	for i, idInList := range extensionList {
		if idInList == extensionID {
			extensionList[i] = extensionList[len(extensionList)-1]
			return extensionList[:len(extensionList)-1]
		}
	}
	return extensionList
}

func writeIfNotExists(name string, content []byte) error {
	exists, err := uio.FileExists(name)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if !exists {
		if err := os.MkdirAll(filepath.Dir(name), uio.FileModeURWXGRWXO); err != nil {
			return uerror.WithStackTrace(err)
		}
		if err := os.WriteFile(name, []byte(content), uio.FileModeURWGRWO); err != nil {
			return uerror.WithStackTrace(err)
		}
	}
	return nil
}

func ensureExists(name string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(name), uio.FileModeURWXGRWXO); err != nil {
		return uerror.WithStackTrace(err)
	}
	if err := os.WriteFile(name, content, uio.FileModeURWGRWO); err != nil {
		return uerror.WithStackTrace(err)
	}
	return nil
}

func ensureExistsFrom(name, srcFile string) error {
	src, err := os.Open(srcFile)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if err := os.MkdirAll(filepath.Dir(name), uio.FileModeURWXGRWXO); err != nil {
		return uerror.WithStackTrace(err)
	}
	dst, err := os.OpenFile(name, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, uio.FileModeURWGRWO)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		return uerror.WithStackTrace(err)
	}
	return nil
}

func writePortSettings(instanceDir string, allInstances []ProfileInstance) error {
	// There's no need to compensate for the currently starting
	// instance in port calculation because "allInstances" is
	// expected to reflect the state before the instance was marked
	// as started.

	runningInstances := 0
	for _, instance := range allInstances {
		if instance.UsagePID != nil {
			runningInstances++
		}
	}
	socksPort := 9150 + 10*runningInstances
	controlPort := 9151 + 10*runningInstances

	profileDir := filepath.Join(instanceDir, relativeProfilePath)
	if err := os.MkdirAll(profileDir, uio.FileModeURWXGRWXO); err != nil {
		return uerror.WithStackTrace(err)
	}

	userJSFile, err := os.OpenFile(filepath.Join(profileDir, "user.js"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, uio.FileModeURWGRWO)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	defer userJSFile.Close()

	if _, err := fmt.Fprintf(userJSFile, ustring.TrimIndentation(`
		user_pref("network.proxy.socks_port", %d);
		user_pref("extensions.torlauncher.control_port", %d);
	`), controlPort, socksPort); err != nil {
		return err
	}

	return nil
}

func setUpBindMounts(instanceDir string) (cleanup func(), err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	cleanUpCache, err := bindMount(cache, filepath.Join(instanceDir, ".cache"), "torbrowser")
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	cleanUpGPGHomeDir, err := bindMount(home, instanceDir, ".local/share/torbrowser/gnupg_homedir")
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	return func() {
		_ = cleanUpCache()
		_ = cleanUpGPGHomeDir()
	}, nil
}

func bindMount(src string, dst string, commonPath string) (cleanup func() error, err error) {
	fullSrc := filepath.Join(src, commonPath)
	fullDst := filepath.Join(dst, commonPath)

	if err := os.MkdirAll(fullSrc, uio.FileModeURWXGRWXO); err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	if err := os.MkdirAll(fullDst, uio.FileModeURWXGRWXO); err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	bindCmd := exec.Command("bindfs", "--no-allow-other", filepath.Join(src, commonPath), fullDst)
	bindCmd.Stdout = os.Stdout
	bindCmd.Stderr = os.Stderr
	if err := bindCmd.Run(); err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	return func() error {
		umountCmd := exec.Command("umount", fullDst)
		umountCmd.Stdout = os.Stdout
		umountCmd.Stderr = os.Stderr
		if err := umountCmd.Run(); err != nil {
			return uerror.WithStackTrace(err)
		}
		return nil
	}, nil
}

func runFirejail(ctx context.Context, instanceDir string, debugShell bool) (uint, error) {
	firejailArgs := []string{
		"dbus-launch", "firejail", fmt.Sprintf("--private=%s", instanceDir),
	}
	if debugShell {
		firejailArgs = append(firejailArgs, "--noprofile", "fish")
	} else {
		firejailArgs = append(firejailArgs, fmt.Sprint("--profile=", filepath.Join(instanceDir, tblFirejailProfileFileName)), "torbrowser-launcher")
	}

	firejailCmd := exec.CommandContext(ctx, firejailArgs[0], firejailArgs[1:]...)
	firejailCmd.Env = append(os.Environ(), "XDG_CACHE_HOME=")
	firejailCmd.Stdin = os.Stdin
	firejailCmd.Stdout = os.Stdout
	firejailCmd.Stderr = os.Stderr

	if err := firejailCmd.Run(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			return uint(err.ExitCode()), nil
		}
		return 0, uerror.WithStackTrace(err)
	}

	return 0, nil
}
