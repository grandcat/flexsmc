package smc

import (
	"errors"
	"time"

	"github.com/golang/glog"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	"golang.org/x/net/context"
)

// Mock to demonstrate the API

const ParallelSessions int = 2

var (
	ErrInvTransition = errors.New("invalid transition")
)

type smcConnectorMock struct {
	readyWorkers chan struct{}
}

func newSMCConnectorMock(socket string) Connector {
	// Do not need the socket, but keep it the same for all implementations.
	_ = socket

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
	id  string
	// For returning our resources, send struct{}{}.
	done chan<- struct{}

	// XXX: protect phase with mutex? doPrepare and doSession run async.
	phase      pbJob.SMCCmd_Phase
	tearedDown bool
}

const initPhase = -1

func (s *smcSessionMock) Init(ctx context.Context, id string) error {
	s.ctx = ctx
	s.id = id
	s.phase = initPhase

	return nil
}

func (s *smcSessionMock) ID() string {
	return s.id
}

func (s *smcSessionMock) NextCmd(in *pbJob.SMCCmd) (out *pbJob.CmdResult, more bool) {
	defer s.condTearDown()
	more = true

	if err := s.validateSession(in); err != nil {
		out, more = reportError(ErrSessionID)
		// SMCCmd_ABORT is irreversible. Consequently, the session is teared down.
		s.phase = pbJob.SMCCmd_ABORT
		return
	}

	switch cmd := in.Payload.(type) {
	case *pbJob.SMCCmd_Prepare:
		if s.allowPhaseTransition(pbJob.SMCCmd_PREPARE) {
			glog.V(3).Infoln("Participants:", cmd.Prepare.Participants)
			out = s.doPrepare(cmd.Prepare)
			more = true
		}

	case *pbJob.SMCCmd_Session:
		if s.allowPhaseTransition(pbJob.SMCCmd_SESSION) {
			glog.V(3).Infoln("Session phase:", cmd.Session)
			out = s.doSession(cmd.Session)
			more = false
		}

	default:
		out, more = reportError(ErrInvTransition)
		s.phase = pbJob.SMCCmd_ABORT
	}
	// Abort on previously noticed invalid phase transition
	if s.phase == pbJob.SMCCmd_ABORT || out == nil {
		out, more = reportError(ErrInvTransition)
	}
	return
}

func (s *smcSessionMock) doPrepare(info *pbJob.PreparePhase) *pbJob.CmdResult {
	res := &pbJob.CmdResult{
		Status: pbJob.CmdResult_SUCCESS,
		Msg:    "->proto.Prepare: nice, but I am stupid",
	}
	// We're doing hard work :)
	time.Sleep(time.Second * 5)

	return res
}

func (s *smcSessionMock) doSession(info *pbJob.SessionPhase) *pbJob.CmdResult {
	res := &pbJob.CmdResult{
		Status: pbJob.CmdResult_SUCCESS,
		Msg:    "->proto.Session: nice, but I am stupid",
		Result: &pbJob.SMCResult{Res: 1234},
	}
	// We're doing hard work :)
	time.Sleep(time.Second * 5)

	// Already tear down here to give the communication layer a chance bringing up
	// another SMC quickly. By overlapping the sessions a bit, processing a batch
	// of SMC jobs should be faster.
	s.TearDown()

	return res
}

func (s *smcSessionMock) releaseResources() {
	if s.tearedDown == false {
		// Invalidate session and release worker resource.
		s.tearedDown = true
		s.done <- struct{}{}
	}
}

func (s *smcSessionMock) TearDown() {
	s.phase = pbJob.SMCCmd_FINISH
	s.condTearDown()
}

// condTearDown releases resources to the pool in case of reaching an invalid or final state.
// No further commands expected.
func (s *smcSessionMock) condTearDown() {
	if s.phase >= pbJob.SMCCmd_FINISH {
		s.releaseResources()
	}
}

func (s *smcSessionMock) validateSession(in *pbJob.SMCCmd) error {
	if s.id != in.SessionID {
		return ErrSessionID
	}
	return nil
}

func (s *smcSessionMock) allowPhaseTransition(newPhase pbJob.SMCCmd_Phase) bool {
	allow := false
	// Verify A -> B transition by checking the reverse direction: means, from B.
	// which were valid states A to have reached this goal.
	switch newPhase {
	case pbJob.SMCCmd_PREPARE:
		switch s.phase {
		case initPhase, pbJob.SMCCmd_PREPARE:
			allow = true
		}

	case pbJob.SMCCmd_SESSION:
		switch s.phase {
		case pbJob.SMCCmd_PREPARE:
			allow = true
		}

		// We do not need to handle the ABORT case separately as the SESSION phase
		// is already the last valid state. There is no way back.
	}
	// Update phase
	if allow {
		s.phase = newPhase
	} else {
		s.phase = pbJob.SMCCmd_ABORT
	}

	return allow
}

func (m *smcSessionMock) Err() error {
	return nil
}
