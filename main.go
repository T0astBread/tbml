package main

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	uerror "t0ast.cc/tbml/util/error"
	uio "t0ast.cc/tbml/util/io"
)

const defaultTBLSettings = `
{"installed": false, "download_over_tor": true, "tor_socks_address": "127.0.0.1:9050", "mirror": "https://dist.torproject.org/", "force_en-US": false}
`

//go:embed files/miniFox@t0ast.cc.xpi
var miniFoxContent []byte

//go:embed files/userChrome.css
var userChromeContent []byte

//go:embed files/user.js
var userJSContent []byte

func main() {
	os.Exit(int(run()))
}

func run() uint {
	if len(os.Args) < 2 {
		panic("Must specify an instance directory")
	}
	instanceDir := os.Args[1]

	ctx := context.Background()

	cleanUpPIDFile, err := setUpPIDFile(instanceDir)
	uerror.ErrPanic(err)
	defer cleanUpPIDFile()

	uerror.ErrPanic(ensureFiles(instanceDir))

	cleanUpBindMounts, err := setUpBindMounts(instanceDir)
	uerror.ErrPanic(err)
	defer cleanUpBindMounts()

	exitCode, err := runFirejail(ctx, instanceDir, os.Args[2:])
	uerror.ErrPanic(err)
	return exitCode
}

func setUpPIDFile(instanceDir string) (cleanup func(), err error) {
	if err := os.MkdirAll(instanceDir, uio.FileModeURWXGRWXO); err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	pidFilePath := filepath.Join(instanceDir, "pidfile")

	pidFileExists, err := uio.FileExists(pidFilePath)
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	if pidFileExists {
		return nil, fmt.Errorf("The instance at %s is already running", instanceDir)
	}

	fileContent := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(pidFilePath, []byte(fileContent), uio.FileModeURWGRWO); err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	return func() {
		os.Remove(pidFilePath)
	}, nil
}

func ensureFiles(instanceDir string) error {
	tblSettingsPath := filepath.Join(instanceDir, ".config/torbrowser/settings.json")
	if err := writeIfNotExists(tblSettingsPath, []byte(defaultTBLSettings)); err != nil {
		return uerror.WithStackTrace(err)
	}

	profileDir := filepath.Join(instanceDir, ".local/share/torbrowser/tbb/x86_64/tor-browser_en-US/Browser/TorBrowser/Data/Browser/profile.default")
	if err := ensureExists(filepath.Join(profileDir, "extensions/miniFox@t0ast.cc.xpi"), miniFoxContent); err != nil {
		return uerror.WithStackTrace(err)
	}
	if err := ensureExists(filepath.Join(profileDir, "chrome/userChrome.css"), userChromeContent); err != nil {
		return uerror.WithStackTrace(err)
	}
	if err := ensureExists(filepath.Join(profileDir, "user.js"), userJSContent); err != nil {
		return uerror.WithStackTrace(err)
	}

	return nil
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

func setUpBindMounts(instanceDir string) (cleanup func(), err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}

	cleanUpCache, err := bindMount(home, instanceDir, ".cache/torbrowser")
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

func runFirejail(ctx context.Context, instanceDir string, restArgs []string) (uint, error) {
	firejailArgs := []string{
		"dbus-launch", "firejail", fmt.Sprintf("--private=%s", instanceDir),
	}
	if len(restArgs) > 0 && restArgs[0] == "--debug" {
		firejailArgs = append(firejailArgs, "--noprofile", "fish")
	} else {
		firejailArgs = append(firejailArgs, "--profile=../torbrowser-launcher.profile", "torbrowser-launcher")
		firejailArgs = append(firejailArgs, restArgs...)
	}

	firejailCmd := exec.CommandContext(ctx, firejailArgs[0], firejailArgs[1:]...)
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
