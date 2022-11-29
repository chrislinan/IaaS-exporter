package errorutil

import (
	"fmt"
	"math/rand"
	"reflect"
	"runtime"
	"time"

	"github.com/go-logr/logr"
)

var RetryOnError = RetryOnErr
var RetryOnErrorWithLog = RetryOnErrWithLog

type stackTrace struct {
	functionName string
	fileName     string
	lineNumber   int
}

// RetryOnErr retries a function until the maximum number of attempts is
// reached, or no error is returned.
func RetryOnErr(f func() error, maxAttempts uint8, sleep time.Duration) error {
	for attempt := uint8(1); attempt <= maxAttempts; attempt++ {
		err := f()
		if err == nil {
			return nil
		}

		if attempt == maxAttempts {
			// Return original error.
			return err
		}

		// Add some randomness to prevent thundering herd problem.
		jitter := time.Duration(rand.Int63n(int64(sleep)))
		sleep += jitter / 2

		time.Sleep(sleep)

		// Try again with exponential backoff.
		sleep = 2 * sleep
	}

	return nil
}

func RetryOnErrWithLog(f func() error, maxAttempts uint8, sleep time.Duration, logger logr.Logger) error {
	for attempt := uint8(1); attempt <= maxAttempts; attempt++ {
		err := f()
		if err == nil {
			return nil
		}

		if attempt == maxAttempts {
			stackTrace := stackTraceForFunc(f)
			logger.Error(err, fmt.Sprintf(
				"function: %s in file: %s in line: %d failed after %d attempts",
				stackTrace.functionName,
				stackTrace.fileName,
				stackTrace.lineNumber,
				maxAttempts,
			))

			// Return original error.
			return err
		}

		// Add some randomness to prevent thundering herd problem.
		jitter := time.Duration(rand.Int63n(int64(sleep)))
		sleep += jitter / 2

		time.Sleep(sleep)

		// Try again with exponential backoff.
		sleep = 2 * sleep
	}

	return nil
}

func stackTraceForFunc(i interface{}) *stackTrace {
	function := runtime.FuncForPC(reflect.ValueOf(i).Pointer())
	entry := function.Entry()
	fileName, lineNumber := function.FileLine(entry)
	functionName := function.Name()
	return &stackTrace{
		functionName: functionName,
		fileName:     fileName,
		lineNumber:   lineNumber,
	}
}
