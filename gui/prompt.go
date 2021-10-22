package gui

import (
	"context"
	"os/exec"
	"strings"

	uerror "t0ast.cc/tbml/util/error"
)

func Prompt(ctx context.Context, items []string, prompt string, matchExact bool) (*string, error) {
	rofiArgs := []string{"-dmenu", "-p", prompt}
	if matchExact {
		rofiArgs = append(rofiArgs, "-no-custom")
	}

	input := strings.Join(items, "\n")

	rofiCmd := exec.CommandContext(ctx, "rofi", rofiArgs...)
	rofiCmd.Stdin = strings.NewReader(input)
	out, err := rofiCmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil, nil
		}
		return nil, uerror.WithStackTrace(err)
	}

	outStr := strings.TrimSuffix(string(out), "\n")
	return &outStr, nil
}
