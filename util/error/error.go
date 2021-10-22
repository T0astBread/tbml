package error

import (
	"errors"
	"fmt"
	"runtime"
)

type ErrorWithExitCode struct {
	ExitCode uint
	Wrapped  error
}

func (e ErrorWithExitCode) Error() string {
	return e.Wrapped.Error()
}

func (e ErrorWithExitCode) Unwrap() error {
	return e.Wrapped
}

func WithExitCode(exitCode uint, err error) error {
	if err, ok := err.(ErrorWithExitCode); ok {
		if err.ExitCode == exitCode {
			return err
		}
	}
	return ErrorWithExitCode{
		ExitCode: exitCode,
		Wrapped:  err,
	}
}

func GetExitCode(err error) (exitCode uint, hasExitCode bool) {
	if err, ok := err.(ErrorWithExitCode); ok {
		return err.ExitCode, true
	}
	if err := errors.Unwrap(err); err != nil {
		return GetExitCode(err)
	}
	return 0, false
}

// ErrorWithStackTrace is an error that attaches a stack trace to its
// message.
type ErrorWithStackTrace struct {
	StackTrace string
	Wrapped    error
}

// Error returns this error's message.
func (s ErrorWithStackTrace) Error() string {
	return fmt.Sprintf("%v\n\n%s\nEND OF StackTraceError", s.Wrapped, s.StackTrace)
}

// Unwrap returns the underlying error of this error.
func (s ErrorWithStackTrace) Unwrap() error {
	return s.Wrapped
}

// WithStackTrace attaches a stack trace to the error, if it does not
// already contain one.
func WithStackTrace(err error) error {
	if err == nil {
		return nil
	}
	if hasStackTrace(err) {
		return err
	}
	st := make([]byte, 1<<16)
	n := runtime.Stack(st, true)
	return ErrorWithStackTrace{
		Wrapped:    err,
		StackTrace: string(st[:n]),
	}
}

func hasStackTrace(err error) bool {
	for err != nil {
		if _, ok := err.(ErrorWithStackTrace); ok {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

func StackTracef(format string, a ...interface{}) error {
	return WithStackTrace(fmt.Errorf(format, a...))
}

// ErrPanic panics if an error is given.
func ErrPanic(err error) {
	if err != nil {
		panic(err)
	}
}
