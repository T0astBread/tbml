package internal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWriteInstanceData(t *testing.T) {
	tmpDir, err := os.MkdirTemp(os.TempDir(), "tbml-test-*")
	assert.NoError(t, err)
	defer assert.NoError(t, os.RemoveAll(tmpDir))

	profile := ProfileConfiguration{
		Label: "test",
	}
	config := Configuration{
		ProfilePath: tmpDir,
		Profiles:    []ProfileConfiguration{profile},
	}
	ul := "test-usage"
	instance := ProfileInstance{
		InstanceLabel: "test-1",
		ProfileLabel:  "test",
		UsageLabel:    &ul,
	}

	instanceDataFile := filepath.Join(tmpDir, "test-1/profile-instance.json")
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
