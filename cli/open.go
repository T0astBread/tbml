package cli

import (
	"errors"
	"fmt"
	"net/url"

	"t0ast.cc/tbml/gui"
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

	if cmd.Topic == "" {
		topics := internal.GetTopics(instances)
		topic, err := gui.Prompt(ctx.Context, topics, "Topic", false)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if topic == nil {
			return errors.New("No topic selected")
		}
		cmd.Topic = *topic
	}

	isNewTopic := internal.IsNewTopic(instances, cmd.Topic)
	if !isNewTopic {
		return errors.New("Sorry, opening new tabs in an existing topic is not supported yet :<")
	}

	if cmd.Profile == "" && isNewTopic {
		profileLabels := internal.GetProfileLabels(ctx.Config)
		profile, err := gui.Prompt(ctx.Context, profileLabels, "Profile", true)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if profile == nil {
			return errors.New("No profile selected")
		}
		cmd.Profile = *profile
	}

	profile := internal.FindProfileByLabel(ctx.Config, cmd.Profile)
	if profile == nil {
		return fmt.Errorf("Profile %s does not exist", cmd.Profile)
	}

	bestInstance := internal.GetBestInstance(*profile, instances)
	fmt.Println("Best:", bestInstance.InstanceLabel)

	bestInstance.UsageLabel = &cmd.Topic

	exitCode, err := internal.StartInstance(ctx.Context, *profile, bestInstance, instances, ctx.ConfigDir, cmd.Debug)
	if err != nil {
		return uerror.WithExitCode(exitCode, uerror.WithStackTrace(err))
	}

	return nil
}
