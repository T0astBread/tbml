package main

import (
	"fmt"
	"os"

	"t0ast.cc/tbml/cli"
	uerror "t0ast.cc/tbml/util/error"
)

func main() {
	err := cli.Run(os.Args)
	if exitCode, hasExitCode := uerror.GetExitCode(err); hasExitCode {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(int(exitCode))
	}
	uerror.ErrPanic(err)
}
