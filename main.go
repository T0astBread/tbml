package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	uerror "t0ast.cc/tbml/util/error"
)

func main() {
	ctx := context.Background()
	configFile := "testdata/config.json"

	config, err := ReadConfiguration(configFile)
	uerror.ErrPanic(err)

	instances, err := GetProfileInstances()
	uerror.ErrPanic(err)

	for _, instance := range instances {
		fmt.Println(instance.ProfileLabel, instance.InstanceLabel, instance.UsageLabel, instance.UsagePID)
	}

	profile := config[0]
	bestInstance := GetBestInstance(profile, instances)
	fmt.Println("Best:", bestInstance.InstanceLabel)

	exitCode, err := StartInstance(ctx, profile, bestInstance, instances, filepath.Dir(configFile), os.Args[1:])
	uerror.ErrPanic(err)

	os.Exit(int(exitCode))
}
