package debughelper

import "os"

// debugModeEnabled controls the evaluation of debug packets' isntructions.
// It is disabled by default to prevent abusing the peers.
var DebugModeEnabled = false

func init() {
	if os.Getenv("FLEX_DEBUG_MODE") == "1" {
		DebugModeEnabled = true
	}
}

// EnableDebugMode activates multiple debug functions that can render a peer unsafe or
// reveal its privacy.
// Call this function prior to any other orchestration call if needed.
func EnableDebugMode() {
	DebugModeEnabled = true
}
