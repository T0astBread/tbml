package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	uerror "t0ast.cc/tbml/util/error"
)

func ReadConfiguration(configFile string) (config []ProfileConfiguration, configDir string, err error) {
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		return nil, "", uerror.WithStackTrace(err)
	}
	return config, filepath.Dir(configFile), json.Unmarshal(configBytes, &config)
}

func GetProfileInstances() ([]ProfileInstance, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	dirEntries, err := os.ReadDir(cacheDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []ProfileInstance{}, nil
	}
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	instances := []ProfileInstance{}
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			return nil, uerror.StackTracef("Non-directory entry found in %s: %s", cacheDir, dirEntry.Name())
		}
		instanceDataBytes, err := os.ReadFile(filepath.Join(cacheDir, dirEntry.Name(), "profile-instance.json"))
		if err != nil {
			return nil, uerror.WithStackTrace(err)
		}
		var instanceData ProfileInstance
		if err := json.Unmarshal(instanceDataBytes, &instanceData); err != nil {
			return nil, uerror.StackTracef("Failed to unmarshal data for profile in %s: %w", dirEntry.Name(), err)
		}
		instances = append(instances, instanceData)
	}
	return instances, nil
}

func FindProfileByLabel(config []ProfileConfiguration, profileLabel string) *ProfileConfiguration {
	for _, profile := range config {
		if profile.Label == profileLabel {
			return &profile
		}
	}
	return nil
}

func GetProfileLabels(config []ProfileConfiguration) []string {
	labels := make([]string, 0, len(config))
	for _, profile := range config {
		labels = append(labels, profile.Label)
	}
	return labels
}

func GetTopics(instances []ProfileInstance) []string {
	topics := []string{}
	for _, instance := range instances {
		if instance.UsageLabel != nil {
			_topic := *instance.UsageLabel // get an unchanging reference to "instance.Topic"
			topics = append(topics, _topic)
		}
	}
	return topics
}

func IsNewTopic(instances []ProfileInstance, topic string) bool {
	for _, instance := range instances {
		if instance.UsageLabel != nil && topic == *instance.UsageLabel {
			return false
		}
	}
	return true
}

func GetBestInstance(profile ProfileConfiguration, instances []ProfileInstance) ProfileInstance {
	instancesForProfile := 0
	var oldestFreeInstance *ProfileInstance
	for _, instance := range instances {
		if instance.ProfileLabel != profile.Label {
			continue
		}
		instancesForProfile++
		if instance.UsagePID != nil {
			continue
		}
		if oldestFreeInstance == nil || instance.Created.Before(oldestFreeInstance.Created) {
			_inst := instance // create an unchanging referece to "instance"
			oldestFreeInstance = &_inst
		}
	}

	if oldestFreeInstance == nil {
		return ProfileInstance{
			InstanceLabel: fmt.Sprintf("%s-%d", profile.Label, instancesForProfile+1),
			ProfileLabel:  profile.Label,
		}
	} else {
		return *oldestFreeInstance
	}
}
