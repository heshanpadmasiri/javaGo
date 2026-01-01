// Package diagnostics represent utiltiy methods for diagnostics messages
package diagnostics

import (
	"fmt"
	"os"
)

// Fatal prints a fatal error message and exits if err is not nil
func Fatal(msg string, err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "Fatal: %s: %v\n", msg, err)
	os.Exit(1)
}
