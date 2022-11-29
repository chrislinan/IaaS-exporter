package errorutil

import (
	"github.com/go-logr/logr"
	"os"
)

// Check prints an error to the standard logger followed by a call to
// os.Exit(1).
func Check(err error, logger logr.Logger) {
	if err != nil {
		logger.Error(err, err.Error())
		os.Exit(1)
	}
}
