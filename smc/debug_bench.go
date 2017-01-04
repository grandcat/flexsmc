package smc

import (
	"context"
	"os"

	"fmt"

	"os/exec"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/benchmark/statistics"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

// debugModeEnabled controls the evaluation of debug packets' isntructions.
// It is disabled by default to prevent abusing the peers.
var debugModeEnabled = false

func init() {
	if os.Getenv("FLEX_DEBUG_MODE") == "1" {
		debugModeEnabled = true
	}
}

func ProcessDebugPhase(in *pbJob.DebugPhase) {
	dbgOpts := in.Options
	if debugModeEnabled == false || dbgOpts == nil {
		return
	}

	// Configure local environment for benchmark.
	if opt, ok := dbgOpts["b.upExpID"]; ok {
		updateSet := opt.GetStr()
		glog.Infof(">>>>>>> DBG: %v [%b]", updateSet, debugModeEnabled)
		statistics.UpdateSetID(updateSet)
		// Upload results to be safe
		UploadBenchStatistics()
	}
}

func UploadBenchStatistics() {
	// Safe to disc before uploading to get really all statistics.
	statistics.GracefulFlush()

	fmt.Println(">>>>>>>> DEBUG: uploading result to kaunas (executing script)")
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "/tmp/uploadStats.sh")
	cmd.Start()
	// err := cmd.Wait()
	// fmt.Println(">>>>>>>> DEBUG: Execution result:", err)
}
