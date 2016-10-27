package smc

import (
	"errors"

	pbJob "github.com/grandcat/flexsmc/proto/job"
	"golang.org/x/net/context"
)

const parallelSessions int = 2

var (
	errInvTransition = errors.New("invalid transition")
)

type FrescoConnect struct {
	readyWorkers chan struct{}
}

func newFrescoConnector() Connector {
	c := &FrescoConnect{
		readyWorkers: make(chan struct{}, parallelSessions),
	}
	// Initially fill in amount of sessions our resources are enough for.
	for i := 0; i < parallelSessions; i++ {
		c.readyWorkers <- struct{}{}
	}

	return c
}

func (con *FrescoConnect) RequestSession(ctx context.Context) (Session, error) {
	select {
	case <-con.readyWorkers:
		// We have some capacities to serve a new SMC session.
		return &frescoSession{
			done: con.readyWorkers,
		}, nil

	case <-ctx.Done():
		// Abort due to context.
		return nil, ctx.Err()
	}
}

type frescoSession struct {
	ctx context.Context
	id  uint64
	// For returning our resources, send struct{}{}.
	done       chan<- struct{}
	tearedDown bool

	phase pbJob.SMCCmd_Phase
}

const startPhase = -1

func (s *frescoSession) Init(ctx context.Context, id uint64) error {
	s.ctx = ctx
	s.id = id
	s.phase = startPhase

	return nil
}

func (s *frescoSession) ID() uint64 {
	return s.id
}

func (s *frescoSession) NextCmd(in *pbJob.SMCCmd) (out *pbJob.CmdResult, more bool) {
	defer s.condTearDown()
	more = true

	if err := s.validateSession(in); err != nil {
		out, more = sendError(ErrSessionID)
		// SMCCmd_ABORT is irreversible. Consequently, the session is teared down.
		s.phase = pbJob.SMCCmd_ABORT
		return

	}

	// Validate transition to new or repeated phase and forward to FRESCO
	if s.allowPhaseTransition(typeToPhase(in)) {
		// TODO: forward message to FRESCO
	}

	// Abort on previously noticed invalid phase transition
	if s.phase == pbJob.SMCCmd_ABORT || out == nil {
		out, more = sendError(errInvTransition)
	}
	return
}

func (s *frescoSession) TearDown() {
	s.phase = pbJob.SMCCmd_FINISH
	s.condTearDown()
}

// condTearDown releases resources to the pool in case of reaching an invalid or final state.
// No further commands expected.
func (s *frescoSession) condTearDown() {
	if s.phase >= pbJob.SMCCmd_FINISH {
		s.releaseResources()
	}
}

func (s *frescoSession) releaseResources() {
	if s.tearedDown == false {
		// Invalidate session and release worker resource.
		s.tearedDown = true
		s.done <- struct{}{}
	}
}

func (s *frescoSession) validateSession(in *pbJob.SMCCmd) error {
	if s.id != in.SessionID {
		return ErrSessionID
	}
	return nil
}

func (s *frescoSession) allowPhaseTransition(newPhase pbJob.SMCCmd_Phase) bool {
	allow := false
	// Verify A -> B transition by checking the reverse direction: means, from B.
	// which were valid states A to have reached this goal.
	switch newPhase {
	case pbJob.SMCCmd_PREPARE:
		switch s.phase {
		case startPhase, pbJob.SMCCmd_SESSION:
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

func typeToPhase(in *pbJob.SMCCmd) pbJob.SMCCmd_Phase {
	switch in.Payload.(type) {
	case *pbJob.SMCCmd_Prepare:
		return pbJob.SMCCmd_PREPARE
	case *pbJob.SMCCmd_Session:
		return pbJob.SMCCmd_SESSION
	default:
		return pbJob.SMCCmd_ABORT
	}
}

func (m *frescoSession) Err() error {
	return nil
}
