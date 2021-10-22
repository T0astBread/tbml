package cli

import (
	"fmt"
	"net/url"

	"t0ast.cc/tbml/internal"
	uerror "t0ast.cc/tbml/util/error"
)

type OpenCmd struct {
	Topic   string   `help:"The topic to open the new tab in" long:"topic" short:"t"`
	Profile string   `help:"The profile to use for opening a new topic; has no effect when not opening a new topic" long:"profile" short:"p"`
	Debug   bool     `help:"Open a debug shell instead of a browser tab"`
	URL     *url.URL `arg:"" help:"A URL to load instead of the new tab page" name:"url" optional:""`
}

func (cmd *OpenCmd) Run(ctx CommandContext) error {
	instances, err := internal.GetProfileInstances()
	if err != nil {
		return err
	}

	for _, instance := range instances {
		fmt.Println(instance.ProfileLabel, instance.InstanceLabel, instance.UsageLabel, instance.UsagePID)
	}

	profile := ctx.Config[0]
	bestInstance := internal.GetBestInstance(profile, instances)
	fmt.Println("Best:", bestInstance.InstanceLabel)

	exitCode, err := internal.StartInstance(ctx.Context, profile, bestInstance, instances, ctx.ConfigDir, cmd.Debug)
	if err != nil {
		return uerror.WithExitCode(exitCode, uerror.WithStackTrace(err))
	}

	return nil
}
