package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/alecthomas/kong"
	"t0ast.cc/tbml/internal"
	uerror "t0ast.cc/tbml/util/error"
	uio "t0ast.cc/tbml/util/io"
)

var ErrNoConfig error = errors.New("No config file found")

var CLI struct {
	ConfigPath string `help:"Path of the configuration file to use (default: ~/.config/tbml/config.json, then /etc/tbml/config.json)" name:"config" optional:"" type:"path"`

	Open OpenCmd `cmd:"" default:"1" help:"Open a new tab (default if no arguments are given)"`

	Ls struct {
	} `cmd:"" help:"List profiles, profile instances and topics"`

	Rm struct {
		Instance string `arg:"" help:"The label of the instance to remove"`
	} `cmd:"" help:"Delete an instance of a profile"`
}

type CommandContext struct {
	Config    internal.Configuration
	ConfigDir string
	Context   context.Context
}

func Run(args []string) error {
	kctx, err := kong.Must(&CLI).Parse(args[1:])
	if err != nil {
		return uerror.WithStackTrace(err)
	}

	config, configDir, err := loadConfig(CLI.ConfigPath)
	if err != nil {
		return uerror.WithStackTrace(err)
	}

	return kctx.Run(CommandContext{
		Config:    config,
		ConfigDir: configDir,
		Context:   context.Background(),
	})
}

func loadConfig(cliPath string) (internal.Configuration, string, error) {
	if cliPath != "" {
		return internal.ReadConfiguration(cliPath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return internal.Configuration{}, "", uerror.WithStackTrace(err)
	}
	homeConfigFile := filepath.Join(home, ".config/tbml/config.json")
	homeConfigFileExists, err := uio.FileExists(homeConfigFile)
	if err != nil {
		return internal.Configuration{}, "", uerror.WithStackTrace(err)
	}
	if homeConfigFileExists {
		return internal.ReadConfiguration(homeConfigFile)
	}

	etcConfigFile := "/etc/tbml/config.json"
	etcConfigFileExists, err := uio.FileExists(etcConfigFile)
	if err != nil {
		return internal.Configuration{}, "", uerror.WithStackTrace(err)
	}
	if etcConfigFileExists {
		return internal.ReadConfiguration(etcConfigFile)
	}

	return internal.Configuration{}, "", uerror.WithStackTrace(ErrNoConfig)
}
