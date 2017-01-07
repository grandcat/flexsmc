package debughelper

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/golang/glog"
	"github.com/grandcat/flexsmc/benchmark/statistics"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

func ProcessDebugPhase(in *pbJob.DebugPhase) (resp *pbJob.CmdResult, moreCmds bool) {
	dbgOpts := in.Options
	if DebugModeEnabled == false || dbgOpts == nil {
		return
	}

	skipSMC := false
	// Configure local environment for benchmark.
	if opt, ok := dbgOpts["b.chgLog"]; ok && len(opt.GetStr()) > 0 {
		newLogPrefix := opt.GetStr()
		glog.Infof(">>>>>>> DBG: change logfile to %s", newLogPrefix)
		statistics.SwitchLog(newLogPrefix)

		skipSMC = true
	}
	if opt, ok := dbgOpts["b.upExpID"]; ok {
		updateSet := opt.GetStr()
		glog.Infof(">>>>>>> DBG: %v [%b]", updateSet, DebugModeEnabled)
		statistics.UpdateSetID(updateSet)
		// Enforce OS configuration.
		ConfigOS()

		skipSMC = true
	}
	if _, ok := dbgOpts["b.upload"]; ok {
		glog.Info(">>>>>>> DBG Upload")
		UploadBenchmarks()

		skipSMC = true
	}
	if _, ok := dbgOpts["skipSMCBackend"]; ok {
		skipSMC = true
	}

	// Generate simple response. This tell the SMC loop we want to leave and
	// there are no further commands to the SMC backend.
	if skipSMC {
		glog.Info(">>>>>>> DBG SkipSMC")
		resp = &pbJob.CmdResult{
			Status: pbJob.CmdResult_SUCCESS_DONE,
			Msg:    "Debug: skipSMC",
		}
		moreCmds = in.MorePhases
	}
	return
}

func ConfigOS() {
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "/tmp/setOSParams.sh")
	cmd.Start()
	// Need to wait in order to release any resources.
	cmd.Wait()
}

func UploadBenchmarks() {
	// There might be some lines buffered in memory. Fetch them for complete dataset.
	statistics.GracefulFlush()

	fmt.Println(">>>>>>>> DEBUG: uploading result to kaunas (executing script)")
	cmd := exec.CommandContext(context.Background(), "/bin/bash", "/tmp/uploadStats.sh")
	cmd.Start()
	// Need to wait in order to release any resources.
	cmd.Wait()
}
