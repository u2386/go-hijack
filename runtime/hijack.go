package runtime

import (
	"fmt"
	"os"
)

type (
	HijackPoint struct {
		Func   string
		Action string
		Val    interface{}
	}
)

var (
	DEBUG = ""
)

func debug(format string, args ...interface{}) {
	if DEBUG != "" {
		fmt.Fprintf(os.Stderr, "GOHIJACK: "+format+"\n", args...)
	}
}