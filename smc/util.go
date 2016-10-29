package smc

import pbJob "github.com/grandcat/flexsmc/proto/job"

func reportError(err error) (out *pbJob.CmdResult, more bool) {
	out = &pbJob.CmdResult{
		Status: pbJob.CmdResult_DENIED,
		Msg:    err.Error(),
	}
	more = false
	return
}
