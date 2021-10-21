package error

import (
	"errors"
	"fmt"
	"runtime"
)

// StackTraceError is an error that attaches a stack trace to its
// message.
type StackTraceError struct {
	Underlying error
	StackTrace string
}

// Error returns this error's message.
func (s StackTraceError) Error() string {
	return fmt.Sprintf("%v\n\n%s\nEND OF StackTraceError", s.Underlying, s.StackTrace)
}

// Unwrap returns the underlying error of this error.
func (s StackTraceError) Unwrap() error {
	return s.Underlying
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
	return StackTraceError{
		Underlying: err,
		StackTrace: string(st[:n]),
	}
}

func hasStackTrace(err error) bool {
	for err != nil {
		if _, ok := err.(StackTraceError); ok {
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
