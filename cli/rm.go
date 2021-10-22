package cli

import (
	"t0ast.cc/tbml/internal"
	uerror "t0ast.cc/tbml/util/error"
)

type RmCmd struct {
	Instance string `arg:"" help:"The label of the instance to remove"`
}

func (cmd *RmCmd) Run(common CommandContext) error {
	instance, err := internal.GetProfileInstance(common.Config, cmd.Instance)
	if err != nil {
		return uerror.WithStackTrace(err)
	}
	return internal.DeleteInstance(common.Config, instance)
}
