package debug

import (
	"io"
	"os"
	"regexp"
	"time"

	uerror "t0ast.cc/tbml/util/error"
)

var debuggerRE *regexp.Regexp = regexp.MustCompile("(?m)^TracerPid:\\s+[1-9]\\d*$")

// WaitForDebugger waits for a debugger to attach to the current
// process.
func WaitForDebugger() {
	for !isBeingDebugged() {
		time.Sleep(250 * time.Millisecond)
	}
}

func isBeingDebugged() bool {
	statusFile, err := os.Open("/proc/self/status")
	uerror.ErrPanic(err)
	statusBytes, err := io.ReadAll(statusFile)
	uerror.ErrPanic(err)
	return debuggerRE.Match(statusBytes)
}
