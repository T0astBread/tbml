package internal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
			desc: "Extensions",

			expectedFiles: map[string]string{
				filepath.Join(relativeProfilePath, "extensions/foo@t0ast.cc.xpi"): "testdata/ensure-files/extensions/foo@t0ast.cc.xpi",
				filepath.Join(relativeProfilePath, "extensions/bar@t0ast.cc.xpi"): "testdata/ensure-files/extensions/bar@t0ast.cc.xpi",
			},
			prepareProfile: func(profile *ProfileConfiguration) {
				profile.ExtensionFiles = []string{
					"extensions/foo@t0ast.cc.xpi",
					"extensions/bar@t0ast.cc.xpi",
				}
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
			desc: "First port ist free",

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
