package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

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
	instances, err := internal.GetProfileInstances(ctx.Config)
	if err != nil {
		return err
	}

	if cmd.Topic == "" {
		topics := internal.GetTopics(instances)
		topic, err := gui.Prompt(ctx.Context, topics, "Topic", false)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if topic == nil || len(strings.TrimSpace(*topic)) == 0 {
			return errors.New("No topic selected")
		}
		cmd.Topic = *topic
	}

	topicInstance := internal.FindInstanceByTopic(instances, cmd.Topic)
	if topicInstance != nil {
		conn, err := internal.ConnectToExternalUnixSocket(ctx.Config, *topicInstance)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		urlStr := ""
		if cmd.URL != nil {
			urlStr = cmd.URL.String()
		}
		if err := internal.SendOpenTabMessage(conn, urlStr); err != nil {
			return uerror.WithStackTrace(err)
		}
		return nil
	}

	if cmd.Profile == "" {
		profileLabels := internal.GetProfileLabels(ctx.Config)
		profile, err := gui.Prompt(ctx.Context, profileLabels, "Profile", true)
		if err != nil {
			return uerror.WithStackTrace(err)
		}
		if profile == nil || len(strings.TrimSpace(*profile)) == 0 {
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

	exitCode, err := internal.StartInstance(ctx.Context, ctx.Config, *profile, bestInstance, instances, ctx.ConfigDir, cmd.URL, cmd.Debug)
	if err != nil {
		return uerror.WithExitCode(exitCode, uerror.WithStackTrace(err))
	}

	return nil
}
