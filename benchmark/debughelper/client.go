package debughelper

import "os"

// debugModeEnabled controls the evaluation of debug packets' isntructions.
// It is disabled by default to prevent abusing the peers.
var debugModeEnabled = false

func init() {
	if os.Getenv("FLEX_DEBUG_MODE") == "1" {
		debugModeEnabled = true
	}
}
