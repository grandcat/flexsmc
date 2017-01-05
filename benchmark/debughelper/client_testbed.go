package debughelper

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/benchmark/statistics"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

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
		// Enforce OS configuration.
		ConfigOS()
	}
	if _, ok := dbgOpts["b.upload"]; ok {
		glog.Info(">>>>>>> DBG Upload")
		UploadBenchmarks()
	}

}

func ConfigOS() {
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "/tmp/setOSParams.sh")
	cmd.Start()
}

func UploadBenchmarks() {
	// There might be some lines buffered in memory. Fetch them for complete dataset.
	statistics.GracefulFlush()

	fmt.Println(">>>>>>>> DEBUG: uploading result to kaunas (executing script)")
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "/tmp/uploadStats.sh")
	cmd.Start()
}
