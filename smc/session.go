package smc

import (
	"time"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

type SMCSession interface {
	// Attach to SMC backend and do initialization if necessary.
	Attach(ctx context.Context, id int) error
	// Prepare phase
	Prepare(info *proto.Prepare) *proto.CmdResult
	// DoSession fires the current session and closes the channel
	// if an error occurres..
	DoSession(info *proto.SessionPhase) <-chan *proto.CmdResult
	// Return a non-nil if an error occurred.
	Err() error
}

var DefaultSMCSession = newSMCMock

type smcMock struct {
}

func newSMCMock() SMCSession {
	return &smcMock{}
}

func (m *smcMock) Attach(ctx context.Context, id int) error {
	return nil
}

func (m *smcMock) Prepare(info *proto.Prepare) *proto.CmdResult {
	res := &proto.CmdResult{
		Status: proto.CmdResult_SUCCESS,
		Msg:    "->proto.Prepare: nice, but I am stupid",
	}
	return res
}

func (m *smcMock) DoSession(info *proto.SessionPhase) <-chan *proto.CmdResult {
	resCh := make(chan *proto.CmdResult)
	go func() {
		res := &proto.CmdResult{
			Status: proto.CmdResult_SUCCESS,
			Msg:    "->proto.Prepare: nice, but I am stupid",
		}
		// We're doing hard work :)
		time.Sleep(time.Second * 1)
		resCh <- res
		close(resCh)
	}()

	return resCh
}

func (m *smcMock) Err() error {
	return nil
}
