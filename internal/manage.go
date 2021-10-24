package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	uerror "t0ast.cc/tbml/util/error"
)

var ErrInstanceInUse error = errors.New("Instance in use")

func ReadConfiguration(configFile string) (config Configuration, configDir string, err error) {
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		return Configuration{}, "", uerror.WithStackTrace(err)
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return Configuration{}, "", uerror.WithStackTrace(err)
	}

	if config.ProfilePath == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			return Configuration{}, "", uerror.WithStackTrace(err)
		}
		config.ProfilePath = filepath.Join(cache, "tbml")
	} else if strings.HasPrefix(config.ProfilePath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return Configuration{}, "", uerror.StackTracef("Failed to expand home directory in profile path: %w", err)
		}
		config.ProfilePath = filepath.Join(home, config.ProfilePath[2:])
	} else if !filepath.IsAbs(config.ProfilePath) {
		config.ProfilePath = filepath.Join(filepath.Dir(configFile), config.ProfilePath)
	}

	return config, filepath.Dir(configFile), nil
}

func GetProfileInstances(config Configuration) ([]ProfileInstance, error) {
	dirEntries, err := os.ReadDir(config.ProfilePath)
	if errors.Is(err, fs.ErrNotExist) {
		return []ProfileInstance{}, nil
	}
	if err != nil {
		return nil, uerror.WithStackTrace(err)
	}
	instances := []ProfileInstance{}
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			return nil, uerror.StackTracef("Non-directory entry found in %s: %s", config.ProfilePath, dirEntry.Name())
		}
		instanceData, err := GetProfileInstance(config, dirEntry.Name())
		if err != nil {
			return nil, uerror.WithStackTrace(err)
		}
		instances = append(instances, instanceData)
	}
	return instances, nil
}

func GetProfileInstance(config Configuration, instanceLabel string) (ProfileInstance, error) {
	instanceDataBytes, err := os.ReadFile(filepath.Join(config.ProfilePath, instanceLabel, "profile-instance.json"))
	if err != nil {
		return ProfileInstance{}, uerror.WithStackTrace(err)
	}
	var instanceData ProfileInstance
	if err := json.Unmarshal(instanceDataBytes, &instanceData); err != nil {
		return ProfileInstance{}, uerror.StackTracef("Failed to unmarshal data for profile in %s: %w", instanceLabel, err)
	}
	return instanceData, nil
}

func DeleteInstance(config Configuration, instance ProfileInstance) error {
	if instance.UsagePID != nil {
		return fmt.Errorf("%w: %s is currently in use by PID %d (topic: %s)", ErrInstanceInUse, instance.InstanceLabel, *instance.UsagePID, *instance.UsageLabel)
	}
	return os.RemoveAll(getInstanceDir(config, instance))
}

func FindProfileByLabel(config Configuration, profileLabel string) *ProfileConfiguration {
	for _, profile := range config.Profiles {
		if profile.Label == profileLabel {
			return &profile
		}
	}
	return nil
}

func GetProfileLabels(config Configuration) []string {
	labels := make([]string, 0, len(config.Profiles))
	for _, profile := range config.Profiles {
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
