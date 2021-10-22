package main

import (
	"fmt"
	"os"

	"t0ast.cc/tbml/cli"
	uerror "t0ast.cc/tbml/util/error"
)

func main() {
	err := cli.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		if exitCode, hasExitCode := uerror.GetExitCode(err); hasExitCode {
			os.Exit(int(exitCode))
		}
		os.Exit(1)
	}
}
