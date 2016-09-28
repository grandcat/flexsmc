package smc

import (
	"time"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

// Connector opens a Session with a SMC backend to track its computation
// result.
type Connector interface {
	// Attach to SMC backend and do initialization if necessary.
	Attach(ctx context.Context, id int) (Session, error)
}

type Session interface {
	// Prepare phase
	Prepare(info *proto.Prepare) <-chan *proto.CmdResult
	// DoSession fires the current session and closes the channel
	// if an error occurres..
	DoSession(info *proto.SessionPhase) <-chan *proto.CmdResult
	// Err returns a non-nil if an error occurred during any phase.
	Err() error
}

var DefaultSMCConnector = newSMCConnector()

type smcConnectorMock struct{}

func newSMCConnector() Connector {
	return &smcConnectorMock{}
}

func (con *smcConnectorMock) Attach(ctx context.Context, id int) (Session, error) {
	return &smcSessionMock{}, nil
}

type smcSessionMock struct{}

func (m *smcSessionMock) Attach(ctx context.Context, id int) error {
	return nil
}

func (m *smcSessionMock) Prepare(info *proto.Prepare) <-chan *proto.CmdResult {
	resCh := make(chan *proto.CmdResult)
	go func() {
		res := &proto.CmdResult{
			Status: proto.CmdResult_SUCCESS,
			Msg:    "->proto.Prepare: nice, but I am stupid",
		}
		// We're doing hard work :)
		time.Sleep(time.Second * 1)
		resCh <- res
		// Only close on error
		// close(resCh)
	}()

	return resCh
}

func (m *smcSessionMock) DoSession(info *proto.SessionPhase) <-chan *proto.CmdResult {
	resCh := make(chan *proto.CmdResult)
	go func() {
		res := &proto.CmdResult{
			Status: proto.CmdResult_SUCCESS,
			Msg:    "->proto.SessionPhase: nice, but I am stupid",
		}
		// We're doing hard work :)
		time.Sleep(time.Second * 1)
		resCh <- res
	}()

	return resCh
}

func (m *smcSessionMock) Err() error {
	return nil
}
