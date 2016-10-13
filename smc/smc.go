package smc

import (
	"errors"
	"log"

	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

// Connector opens a Session with a SMC backend to control and track its computation.
type Connector interface {
	// RequestSession blocks until resources allow for a new Session.
	// The context can abort an ongoing request. This will cause an error.
	RequestSession(ctx context.Context) (Session, error)
}

type Session interface {
	Init(ctx context.Context, id uint64)
	// ID of the current session.
	ID() uint64
	// NextCmd evaluates the input command, forwards it to the SMC backend and sends back
	// the result of the command evaluation.
	// It blocks until the request is processed or the context is done.
	NextCmd(in *proto.SMCCmd) (out *proto.CmdResult, more bool)
	// TearDown finishes a session and frees occupied resources. This should be called
	// if the originator is not interested anymore to keep the SMC reservation alive.
	TearDown()
	// Err returns a non-nil if an error occurred during any phase.
	Err() error
}

var DefaultSMCConnector = newSMCConnector()

var (
	errSessionID     = errors.New("altering session not allowed here")
	errInvTransition = errors.New("invalid transition")
)

// Mock to demonstrate the API

const ParallelSessions int = 2

type smcConnectorMock struct {
	readyWorkers chan struct{}
}

func newSMCConnector() Connector {
	c := &smcConnectorMock{
		readyWorkers: make(chan struct{}, ParallelSessions),
	}
	// Initially fill in amount of sessions our resources are enough for.
	for i := 0; i < ParallelSessions; i++ {
		c.readyWorkers <- struct{}{}
	}

	return c
}

func (con *smcConnectorMock) RequestSession(ctx context.Context) (Session, error) {
	select {
	case <-con.readyWorkers:
		// We have some capacities to serve a new SMC session.
		return &smcSessionMock{
			done: con.readyWorkers,
		}, nil

	case <-ctx.Done():
		// Abort due to context.
		return nil, ctx.Err()
	}
}

type smcSessionMock struct {
	ctx context.Context
	id  uint64
	// For returning our resources, send struct{}{}.
	done chan<- struct{}

	// XXX: protect phase with mutex? doPrepare and doSession run async.
	phase      proto.SMCCmd_Phase
	tearedDown bool
}

const initPhase = -1

func (s *smcSessionMock) Init(ctx context.Context, id uint64) {
	s.ctx = ctx
	s.id = id
	s.phase = initPhase
}

func (s *smcSessionMock) ID() uint64 {
	return s.id
}

func (s *smcSessionMock) NextCmd(in *proto.SMCCmd) (out *proto.CmdResult, more bool) {
	defer s.condTearDown()
	more = true

	if err := s.validateSession(in); err != nil {
		out, more = sendError(errSessionID)
		// SMCCmd_ABORT is irreversible. Consequently, the session is teared down.
		s.phase = proto.SMCCmd_ABORT
		return
	}

	switch cmd := in.Payload.(type) {
	case *proto.SMCCmd_Prepare:
		if s.allowPhaseTransition(proto.SMCCmd_PREPARE) {
			log.Println(">> Participants:", cmd.Prepare.Participants)
			out = s.doPrepare(cmd.Prepare)
			more = true
		}

	case *proto.SMCCmd_Session:
		if s.allowPhaseTransition(proto.SMCCmd_SESSION) {
			log.Println(">> Session phase:", cmd.Session)
			out = s.doSession(cmd.Session)
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

func (s *smcSessionMock) doPrepare(info *proto.Prepare) *proto.CmdResult {
	res := &proto.CmdResult{
		Status: proto.CmdResult_SUCCESS,
		Msg:    "->proto.Prepare: nice, but I am stupid",
	}
	// We're doing hard work :)
	// time.Sleep(time.Second * 1)

	return res
}

func (s *smcSessionMock) doSession(info *proto.SessionPhase) *proto.CmdResult {
	res := &proto.CmdResult{
		Status: proto.CmdResult_SUCCESS,
		Msg:    "->proto.Session: nice, but I am stupid",
	}
	// We're doing hard work :)
	// time.Sleep(time.Second * 1)

	// Already tear down here to give the communication layer a chance bringing up
	// another SMC quickly. By overlapping the sessions a bit, processing a batch
	// of SMC jobs should be faster.
	s.TearDown()

	return res
}

func (s *smcSessionMock) TearDown() {
	s.phase = proto.SMCCmd_FINISH
	s.condTearDown()
}

// condTearDown releases resources to the pool in case of reaching an invalid or final state
// expecting no further commands.
func (s *smcSessionMock) condTearDown() {
	if s.phase >= proto.SMCCmd_FINISH && s.tearedDown == false {
		// Invalidate session.
		s.tearedDown = true
		// Return resources for other session requestors.
		s.done <- struct{}{}
	}
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
		case initPhase, proto.SMCCmd_SESSION:
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

func sendError(err error) (out *proto.CmdResult, more bool) {
	out = &proto.CmdResult{
		Status: proto.CmdResult_DENIED,
		Msg:    err.Error(),
	}
	more = false
	return
}

func (m *smcSessionMock) Err() error {
	return nil
}
