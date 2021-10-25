package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	uio "t0ast.cc/tbml/util/io"
	ustring "t0ast.cc/tbml/util/string"
)

func setUpTestEnvironment(t *testing.T) (config Configuration, profile ProfileConfiguration, instance ProfileInstance, instanceDir string, cleanup func()) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "tbml-test-*")
	assert.NoError(t, err)

	profile = ProfileConfiguration{
		Label: "test",
	}
	config = Configuration{
		ProfilePath: tmpDir,
		Profiles:    []ProfileConfiguration{profile},
	}
	ul := "test-usage"
	instance = ProfileInstance{
		InstanceLabel: "test-1",
		ProfileLabel:  "test",
		UsageLabel:    &ul,
	}

	return config, profile, instance, filepath.Join(tmpDir, instance.InstanceLabel), func() {
		assert.NoError(t, os.RemoveAll(tmpDir))
	}
}

func TestWriteInstanceData(t *testing.T) {
	config, profile, instance, instanceDir, cleanUpEnvironment := setUpTestEnvironment(t)
	defer cleanUpEnvironment()

	instanceDataFile := filepath.Join(instanceDir, "profile-instance.json")
	assert.NoFileExists(t, instanceDataFile)

	cleanUp, err := writeInstanceData(config, profile, instance)
	assert.NoError(t, err)

	assert.FileExists(t, instanceDataFile)

	instanceDataBytes, err := os.ReadFile(instanceDataFile)
	assert.NoError(t, err)
	var actual ProfileInstance
	assert.NoError(t, json.Unmarshal(instanceDataBytes, &actual))

	currentPID := os.Getpid()
	assert.Equal(t, currentPID, *actual.UsagePID)

	assert.True(t, time.Now().Add(-10*time.Second).Before(actual.Created))
	assert.True(t, time.Now().After(actual.Created))
	assert.True(t, time.Now().Add(-10*time.Second).Before(actual.LastUsed))
	assert.True(t, time.Now().After(actual.LastUsed))

	createdBeforeCleanup := actual.Created
	lastUsedBeforeCleanup := actual.LastUsed

	actual.Created = instance.Created
	actual.LastUsed = instance.LastUsed
	actual.UsagePID = instance.UsagePID
	assert.Equal(t, instance, actual)

	assert.NoError(t, cleanUp())
	assert.FileExists(t, instanceDataFile)

	instanceDataBytes, err = os.ReadFile(instanceDataFile)
	assert.NoError(t, err)
	actual = ProfileInstance{}
	assert.NoError(t, json.Unmarshal(instanceDataBytes, &actual))

	assert.Nil(t, actual.UsageLabel)
	assert.Nil(t, actual.UsagePID)

	assert.True(t, time.Now().Add(-10*time.Second).Before(actual.Created))
	assert.True(t, time.Now().After(actual.Created))
	assert.True(t, time.Now().Add(-10*time.Second).Before(actual.LastUsed))
	assert.True(t, time.Now().After(actual.LastUsed))

	assert.True(t, actual.Created.Equal(createdBeforeCleanup))
	assert.True(t, actual.LastUsed.After(lastUsedBeforeCleanup))

	actual.Created = instance.Created
	actual.LastUsed = instance.LastUsed
	actual.UsageLabel = instance.UsageLabel
	assert.Equal(t, instance, actual)
}

func TestEnsureFiles(t *testing.T) {
	testCases := []struct {
		desc string

		expectChangesAreKept bool
		expectedFiles        map[string]string
		prepareProfile       func(profile *ProfileConfiguration)
	}{
		{
			desc: "torbrowser-launcher settings",

			expectChangesAreKept: true,
			expectedFiles: map[string]string{
				".config/torbrowser/settings.json": "torbrowser-launcher-default-settings.json",
			},
		},
		{
			desc: "Firejail profile",

			expectedFiles: map[string]string{
				"torbrowser-launcher.profile": "torbrowser-launcher.profile",
			},
		},
		{
			desc: "userChrome.css",

			expectedFiles: map[string]string{
				filepath.Join(relativeProfilePath, "chrome/userChrome.css"): "testdata/ensure-files/userChrome.css",
			},
			prepareProfile: func(profile *ProfileConfiguration) {
				uc := "userChrome.css"
				profile.UserChromeFile = &uc
			},
		},
		{
			desc: "user.js",

			expectedFiles: map[string]string{
				filepath.Join(relativeProfilePath, "user.js"): "testdata/ensure-files/user.js",
			},
			prepareProfile: func(profile *ProfileConfiguration) {
				uj := "user.js"
				profile.UserJSFile = &uj
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			_, profile, _, instanceDir, cleanUpEnvironment := setUpTestEnvironment(t)
			defer cleanUpEnvironment()

			if tC.prepareProfile != nil {
				tC.prepareProfile(&profile)
			}

			verifyFileContentsFromMap := func(t *testing.T) {
				for k, v := range tC.expectedFiles {
					actualPath := filepath.Join(instanceDir, k)
					assert.FileExists(t, actualPath)
					actualContent, err := os.ReadFile(actualPath)
					assert.NoError(t, err)
					assert.NotEmpty(t, actualContent)
					expectedContent, err := os.ReadFile(v)
					assert.NoError(t, err)
					assert.Equal(t, expectedContent, actualContent)
				}
			}

			t.Run("Write initially", func(t *testing.T) {
				for k := range tC.expectedFiles {
					assert.NoFileExists(t, filepath.Join(instanceDir, k))
				}

				assert.NoError(t, ensureFiles(profile, "testdata/ensure-files", instanceDir))

				verifyFileContentsFromMap(t)
			})
			t.Run("Update existing", func(t *testing.T) {
				changedContent := []byte("This file has been changed")

				for k := range tC.expectedFiles {
					assert.NoError(t, os.WriteFile(filepath.Join(instanceDir, k), changedContent, uio.FileModeURWGRWO))
				}

				assert.NoError(t, ensureFiles(profile, "testdata/ensure-files", instanceDir))

				if tC.expectChangesAreKept {
					for k := range tC.expectedFiles {
						actualPath := filepath.Join(instanceDir, k)
						assert.FileExists(t, actualPath)
						actualContent, err := os.ReadFile(actualPath)
						assert.NoError(t, err)
						assert.Equal(t, changedContent, actualContent)
					}
				} else {
					verifyFileContentsFromMap(t)
				}
			})
		})
	}
}

func TestEnsureExtensions(t *testing.T) {
	testCases := []struct {
		desc string

		extensionsInProfile       []string
		installedExtensionsBefore []string

		installedExtensionsAfter []string
	}{
		{
			desc: "Only new extensions",

			extensionsInProfile: []string{
				"foo@t0ast.cc",
				"bar@t0ast.cc",
			},

			installedExtensionsAfter: []string{
				"foo@t0ast.cc",
				"bar@t0ast.cc",
			},
		},
		{
			desc: "Remove old extension",

			extensionsInProfile: []string{
				"foo@t0ast.cc",
				"bar@t0ast.cc",
			},
			installedExtensionsBefore: []string{
				"baz@t0ast.cc",
			},

			installedExtensionsAfter: []string{
				"foo@t0ast.cc",
				"bar@t0ast.cc",
			},
		},
		{
			desc: "Keep some extensions",

			extensionsInProfile: []string{
				"foo@t0ast.cc",
				"bar@t0ast.cc",
			},
			installedExtensionsBefore: []string{
				"bar@t0ast.cc",
				"baz@t0ast.cc",
			},

			installedExtensionsAfter: []string{
				"foo@t0ast.cc",
				"bar@t0ast.cc",
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			config, profile, instance, instanceDir, cleanUpEnvironment := setUpTestEnvironment(t)
			defer cleanUpEnvironment()

			configDir := "testdata/ensure-extensions"

			extensionsDir := filepath.Join(instanceDir, relativeProfilePath, "extensions")
			assert.NoError(t, os.MkdirAll(extensionsDir, uio.FileModeURWXGRWXO))

			doNotDeleteContent := []byte("Do not delete me plz :>")
			doNotDeletePath := filepath.Join(extensionsDir, "donotdelete@t0ast.cc.xpi")
			assert.NoError(t, os.WriteFile(doNotDeletePath, doNotDeleteContent, uio.FileModeURWGRWO))

			for _, ext := range tC.installedExtensionsBefore {
				srcPath := filepath.Join("testdata/ensure-extensions/extensions", fmt.Sprint(ext, ".xpi"))
				dstPath := filepath.Join(extensionsDir, fmt.Sprint(ext, ".xpi"))
				assert.NoError(t, uio.CopyFile(srcPath, dstPath))

				instance.InstalledExtensions = append(instance.InstalledExtensions, ext)
			}

			for _, ext := range tC.extensionsInProfile {
				profile.ExtensionFiles = append(profile.ExtensionFiles, fmt.Sprint("extensions/", ext, ".xpi"))
			}

			instanceDataBytes, err := json.Marshal(instance)
			assert.NoError(t, err)
			assert.NoError(t, os.MkdirAll(instanceDir, uio.FileModeURWXGRWXO))
			assert.NoError(t, os.WriteFile(filepath.Join(instanceDir, "profile-instance.json"), instanceDataBytes, uio.FileModeURWGRWO))

			assert.NoError(t, ensureExtensions(config, profile, instance.InstanceLabel, configDir, instanceDir))

			assert.FileExists(t, doNotDeletePath)
			doNotDeleteContentAfter, err := os.ReadFile(doNotDeletePath)
			assert.NoError(t, err)
			assert.Equal(t, doNotDeleteContent, doNotDeleteContentAfter)

			instanceAfter, err := GetProfileInstance(config, "test-1")
			assert.NoError(t, err)

			expected := tC.installedExtensionsAfter
			sort.Slice(expected, func(i, j int) bool {
				return expected[i] < expected[j]
			})
			actual := instanceAfter.InstalledExtensions
			sort.Slice(actual, func(i, j int) bool {
				return actual[i] < actual[j]
			})
			assert.Equal(t, expected, actual)

			extensionsAfter := make(map[string]bool)
			for _, ext := range tC.installedExtensionsAfter {
				extensionsAfter[ext] = true
			}
			for _, ext := range tC.installedExtensionsBefore {
				srcPath := filepath.Join("testdata/ensure-extensions/extensions", fmt.Sprint(ext, ".xpi"))
				dstPath := filepath.Join(extensionsDir, fmt.Sprint(ext, ".xpi"))
				if extensionsAfter[ext] {
					assert.FileExists(t, dstPath)
					expectedFileContent, err := os.ReadFile(srcPath)
					assert.NoError(t, err)
					actualFileContent, err := os.ReadFile(dstPath)
					assert.NoError(t, err)
					assert.Equal(t, expectedFileContent, actualFileContent)
				} else {
					assert.FileExists(t, srcPath)
					assert.NoFileExists(t, dstPath)
				}
			}
		})
	}
}

func TestWritePortSettings(t *testing.T) {
	somePid := 1234

	testCases := []struct {
		desc string

		existingUserJSContent string
		expectedControlPort   uint
		expectedSOCKSPort     uint
		extraInstances        []ProfileInstance
	}{
		{
			desc: "First port is free",

			expectedControlPort: 9151,
			expectedSOCKSPort:   9150,
		},
		{
			desc: "Later port",

			expectedControlPort: 9161,
			expectedSOCKSPort:   9160,
			extraInstances: []ProfileInstance{
				{
					InstanceLabel: "test-2",
					ProfileLabel:  "test",
					UsagePID:      &somePid,
				},
			},
		},
		{
			desc: "With existing user.js file",

			existingUserJSContent: ustring.TrimIndentation(`
				// This is an existing user.js file.
				user_pref("foo", "bar");

			`),
			expectedControlPort: 9151,
			expectedSOCKSPort:   9150,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			_, _, instance, instanceDir, cleanUpEnvironment := setUpTestEnvironment(t)
			defer cleanUpEnvironment()

			userJSPath := filepath.Join(instanceDir, relativeProfilePath, "user.js")
			if tC.existingUserJSContent != "" {
				assert.NoError(t, os.MkdirAll(filepath.Dir(userJSPath), uio.FileModeURWXGRWXO))
				assert.NoError(t, os.WriteFile(userJSPath, []byte(tC.existingUserJSContent), uio.FileModeURWGRWO))
			}

			assert.NoError(t, writePortSettings(instanceDir, append(tC.extraInstances, instance)))

			actualUserJS, err := os.ReadFile(userJSPath)
			assert.NoError(t, err)

			expectedUserJS := fmt.Sprintf(ustring.TrimIndentation(`
				%suser_pref("network.proxy.socks_port", %d);
				user_pref("extensions.torlauncher.control_port", %d);
			`), tC.existingUserJSContent, tC.expectedControlPort, tC.expectedSOCKSPort)

			assert.Equal(t, expectedUserJS, string(actualUserJS))
		})
	}
}

func assertIsBindMount(t *testing.T, mountpoint, dst string) {
	mountpointCmd := exec.Command("mountpoint", mountpoint)
	output, err := mountpointCmd.CombinedOutput()
	assert.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("%s is a mountpoint\n", mountpoint), string(output))

	testFileContent := "This should appear in the dst directory."
	assert.NoError(t, os.WriteFile(filepath.Join(mountpoint, "mountpoint-test.txt"), []byte(testFileContent), uio.FileModeURWGRWO))

	testFilePathInDst := filepath.Join(dst, "mountpoint-test.txt")
	assert.FileExists(t, testFilePathInDst)
	testFileContentInDst, err := os.ReadFile(testFilePathInDst)
	assert.NoError(t, err)
	assert.Equal(t, testFileContent, string(testFileContentInDst))
}

func assertNoMountpoint(t *testing.T, mountpoint string) {
	mountpointCmd := exec.Command("mountpoint", mountpoint)
	output, err := mountpointCmd.CombinedOutput()

	assert.Error(t, err)
	if exitErr, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 1, exitErr.ExitCode())
	} else {
		assert.Fail(t, "Error was not an ExitError", err)
	}

	assert.Equal(t, fmt.Sprintf("%s is not a mountpoint\n", mountpoint), string(output))
}

func TestSetUpBindMounts(t *testing.T) {
	testCases := []struct {
		desc string

		cachePath string
	}{
		{
			desc: "Cache outside of home directory",

			cachePath: "tmp-cache",
		},
		{
			desc: "Cache in home directory",

			cachePath: "tmp-home/.cache",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			_, _, _, instanceDir, cleanUpEnvironment := setUpTestEnvironment(t)
			defer cleanUpEnvironment()

			assert.Equal(t, runtime.GOOS, "linux")
			origHome := os.Getenv("HOME")
			origCache := os.Getenv("XDG_CACHE_HOME")
			resetEnv := func() {
				os.Setenv("HOME", origHome)
				os.Setenv("XDG_CACHE_HOME", origCache)
			}
			defer resetEnv()

			tmpHome := filepath.Join(instanceDir, "tmp-home")
			tmpCache := filepath.Join(instanceDir, tC.cachePath)
			os.Setenv("HOME", tmpHome)
			if tC.cachePath == "" {
				os.Unsetenv("XDG_CACHE_HOME")
			} else {
				os.Setenv("XDG_CACHE_HOME", tmpCache)
			}

			home, err := os.UserHomeDir()
			assert.NoError(t, err)
			assert.Equal(t, tmpHome, home)
			gnupgHomedirDst := filepath.Join(home, ".local/share/torbrowser/gnupg_homedir")
			assert.NoError(t, os.MkdirAll(gnupgHomedirDst, uio.FileModeURWXGRWXO))

			cache, err := os.UserCacheDir()
			assert.NoError(t, err)
			assert.Equal(t, tmpCache, cache)
			torbrowserCacheDst := filepath.Join(cache, "torbrowser")
			assert.NoError(t, os.MkdirAll(torbrowserCacheDst, uio.FileModeURWXGRWXO))

			cleanUp, err := setUpBindMounts(instanceDir)
			assert.NoError(t, err)
			defer cleanUp()

			resetEnv()

			torbrowserCacheInstanceDir := filepath.Join(instanceDir, ".cache/torbrowser")
			gnupgHomedirInstanceDir := filepath.Join(instanceDir, ".local/share/torbrowser/gnupg_homedir")

			assertIsBindMount(t, torbrowserCacheInstanceDir, torbrowserCacheDst)
			assertIsBindMount(t, gnupgHomedirInstanceDir, gnupgHomedirDst)

			cleanUp()

			assertNoMountpoint(t, torbrowserCacheInstanceDir)
			assertNoMountpoint(t, gnupgHomedirInstanceDir)
		})
	}
}
