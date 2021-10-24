package internal_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"t0ast.cc/tbml/internal"
)

var uc = "userChrome.css"
var uj = "user.js"

func getConfigurationFixture() internal.Configuration {
	return internal.Configuration{
		Profiles: []internal.ProfileConfiguration{
			{
				ExtensionFiles: []string{
					"extensions/foobar@t0ast.cc.xpi",
				},
				Label:          "test",
				UserChromeFile: &uc,
				UserJSFile:     &uj,
			},
		},
	}
}

func TestReadConfiguration(t *testing.T) {
	testCases := []struct {
		desc string

		configFileName  string
		prepareExpected func(expected *internal.Configuration)
	}{
		{
			desc: "No profile path",

			configFileName: "config-no-profile-path.json",
			prepareExpected: func(expected *internal.Configuration) {
				cache, err := os.UserCacheDir()
				assert.NoError(t, err)
				expected.ProfilePath = filepath.Join(cache, "tbml")
			},
		},
		{
			desc: "Profile path from home",

			configFileName: "config-profile-path-from-home.json",
			prepareExpected: func(expected *internal.Configuration) {
				home, err := os.UserHomeDir()
				assert.NoError(t, err)
				expected.ProfilePath = filepath.Join(home, ".tbml")
			},
		},
		{
			desc: "Profile path from root",

			configFileName: "config-profile-path-from-root.json",
			prepareExpected: func(expected *internal.Configuration) {
				expected.ProfilePath = "/tmp/tbml"
			},
		},
		{
			desc: "Relative profile path",

			configFileName: "config-relative-profile-path.json",
			prepareExpected: func(expected *internal.Configuration) {
				expected.ProfilePath = "testdata/tbml/profiles"
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			expected := getConfigurationFixture()
			tC.prepareExpected(&expected)

			config, configDir, err := internal.ReadConfiguration(filepath.Join("testdata", tC.configFileName))
			assert.NoError(t, err)
			assert.Equal(t, expected, config)
			assert.Equal(t, "testdata", configDir)
		})
	}
}

func TestReadConfigurationNonexistent(t *testing.T) {
	_, _, err := internal.ReadConfiguration("testdata/config-nonexistent.json")
	assert.ErrorIs(t, err, fs.ErrNotExist)
}
