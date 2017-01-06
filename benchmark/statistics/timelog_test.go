package statistics

import "testing"
import pbJob "github.com/grandcat/flexsmc/proto/job"

func Test_enter(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"1enter"},
		{"2enter"},
		{"1enter"},
		{"2enter"},
		{"1enter"},
		{"2enter"},
		{"1enter"},
		{"2enter"},
		{"1enter"},
		{"2enter"},
		{"1enter"},
		{"2enter"},
	}
	for range tests {
		s := StartTrack()
		j := &pbJob.SMCCmd{
			Payload: &pbJob.SMCCmd_Prepare{Prepare: &pbJob.PreparePhase{}},
		}
		G(0).End(j.GetPayload(), s, "optional")
	}
	timeLogger.flush()
}

func Test_switchLogfile(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"prefix.one."},
		{"prefix.two."},
	}
	for _, t := range tests {
		SwitchLog(t.name)
		// Some entries
		s := StartTrack()
		j := &pbJob.SMCCmd{
			Payload: &pbJob.SMCCmd_Prepare{Prepare: &pbJob.PreparePhase{}},
		}
		G(0).End(j.GetPayload(), s, "entry for"+t.name)
	}
	timeLogger.flush()
}
