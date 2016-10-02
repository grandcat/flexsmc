package smc

import (
	"errors"
	"log"
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
	NextCmd(in *proto.SMCCmd) (out <-chan *proto.CmdResult, more bool)
	// Err returns a non-nil if an error occurred during any phase.
	Err() error
}

var DefaultSMCConnector = newSMCConnector()

var (
	errSessionID     = errors.New("altering session not allowed here")
	errInvTransition = errors.New("invalid transition")
)

// Mock to demonstrate the API

type smcConnectorMock struct{}

func newSMCConnector() Connector {
	return &smcConnectorMock{}
}

func (con *smcConnectorMock) Attach(ctx context.Context, id uint64) (Session, error) {
	return &smcSessionMock{id: id}, nil
}

type smcSessionMock struct {
	id    uint64
	phase proto.SMCCmd_Phase
}

func (s *smcSessionMock) ID() uint64 {
	return s.id
}

func (s *smcSessionMock) NextCmd(in *proto.SMCCmd) (out <-chan *proto.CmdResult, more bool) {
	more = true

	if err := s.validateSession(in); err != nil {
		out, more = sendError(errSessionID)
		s.phase = proto.SMCCmd_ABORT
		return
	}

	switch cmd := in.Payload.(type) {
	case *proto.SMCCmd_Prepare:
		if s.allowPhaseTransition(proto.SMCCmd_PREPARE) {
			log.Println(">> Participants:", cmd.Prepare.Participants)
			out = s.DoPrepare(cmd.Prepare)
			more = true
		}

	case *proto.SMCCmd_Session:
		if s.allowPhaseTransition(proto.SMCCmd_SESSION) {
			log.Println(">> Session phase:", cmd.Session)
			out = s.DoSession(cmd.Session)
			more = false
		}

	default:
		out, more = sendError(errInvTransition)
		s.phase = proto.SMCCmd_ABORT
	}
	// Abort on previously noticed invalid phase transition
	if s.phase == proto.SMCCmd_ABORT || out == nil {
		out, more = sendError(errInvTransition)
	}
	return
}

func (s *smcSessionMock) DoPrepare(info *proto.Prepare) <-chan *proto.CmdResult {
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

func (s *smcSessionMock) DoSession(info *proto.SessionPhase) <-chan *proto.CmdResult {
	// Update current phase
	s.phase = proto.SMCCmd_SESSION

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

func (s *smcSessionMock) validateSession(in *proto.SMCCmd) error {
	if s.id != in.SessionID {
		return errSessionID
	}
	return nil
}

func (s *smcSessionMock) allowPhaseTransition(newPhase proto.SMCCmd_Phase) bool {
	allow := false
	// Verify A -> B transition by checking the reverse direction: means, from B.
	// which were valid states A to have reached this goal.
	switch newPhase {
	case proto.SMCCmd_PREPARE:
		switch s.phase {
		case proto.SMCCmd_INIT, proto.SMCCmd_SESSION:
			allow = true
		}

	case proto.SMCCmd_SESSION:
		switch s.phase {
		case proto.SMCCmd_PREPARE:
			allow = true
		}

		// We do not need to handle the ABORT case separately as the SESSION phase
		// is already the last valid state. There is no way back.
	}
	// Update phase
	if allow {
		s.phase = newPhase
	} else {
		s.phase = proto.SMCCmd_ABORT
	}

	return allow
}

func sendResponse(resp proto.CmdResult) <-chan *proto.CmdResult {
	resCh := make(chan *proto.CmdResult)
	go func() {
		resCh <- &resp
	}()
	return resCh
}

func sendError(err error) (out <-chan *proto.CmdResult, more bool) {
	out = sendResponse(proto.CmdResult{
		Status: proto.CmdResult_DENIED,
		Msg:    err.Error(),
	})
	more = false
	return
}

func (m *smcSessionMock) Err() error {
	return nil
}
