package smc

import (
	"time"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

// Connector opens a Session with a SMC backend to control and track its computation.
type Connector interface {
	// Attach to SMC backend and do initialization if necessary.
	Attach(ctx context.Context, id uint64) (Session, error)
}

type Session interface {
	ID() uint64
	// Prepare phase
	Prepare(info *proto.Prepare) <-chan *proto.CmdResult
	// DoSession fires the current session and closes the channel
	// if an error occurres..
	DoSession(info *proto.SessionPhase) <-chan *proto.CmdResult
	// Err returns a non-nil if an error occurred during any phase.
	Err() error
}

var DefaultSMCConnector = newSMCConnector()

// Mock to demonstrate the API

type smcConnectorMock struct{}

func newSMCConnector() Connector {
	return &smcConnectorMock{}
}

func (con *smcConnectorMock) Attach(ctx context.Context, id uint64) (Session, error) {
	return &smcSessionMock{id: id}, nil
}

type smcSessionMock struct {
	id uint64
}

func (m *smcSessionMock) ID() uint64 {
	return m.id
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
