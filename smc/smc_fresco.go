package smc

import (
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"fmt"

	"github.com/golang/glog"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	proto "github.com/grandcat/flexsmc/proto/smc"
	"golang.org/x/net/context"
)

const parallelSessions int = 2

var (
	errInvalidCmd = errors.New("invalid command")
)

type FrescoConnect struct {
	backendSocket string
	readyWorkers  chan struct{}
}

func newFrescoConnector(socket string) Connector {
	c := &FrescoConnect{
		backendSocket: socket,
		readyWorkers:  make(chan struct{}, parallelSessions),
	}
	// Initially fill in amount of sessions our resources are enough for.
	for i := 0; i < parallelSessions; i++ {
		c.readyWorkers <- struct{}{}
	}

	return c
}

func (con *FrescoConnect) RequestSession(ctx context.Context) (Session, error) {
	// Connect to local Fresco instance to see if it is present at all
	cc, err := DialSocket(con.backendSocket)
	if err != nil {
		return nil, err
	}
	client := proto.NewSMCClient(cc)

	select {
	case <-con.readyWorkers:
		// We have some capacities to serve a new SMC session.
		return &frescoSession{
			conn:   cc,
			client: client,
			done:   con.readyWorkers,
		}, nil

	case <-ctx.Done():
		// Abort due to context.
		return nil, ctx.Err()
	}
}

func (con *FrescoConnect) ResetAll(ctx context.Context) error {
	cc, err := DialSocket(con.backendSocket)
	if err != nil {
		return err
	}
	defer cc.Close()
	cl := proto.NewSMCClient(cc)

	resp, err := cl.ResetAll(ctx, &proto.FilterArgs{})
	if err != nil {
		return err
	}
	if (resp.Status & pbJob.CmdResult_ALL_ERROR_CLASSES) > 0 {
		return fmt.Errorf("could not reset all sessions: %s", resp.Msg)
	}

	glog.V(3).Infof("ResetAll successful.")
	return nil
}

type frescoSession struct {
	ctx    context.Context
	conn   *grpc.ClientConn
	client proto.SMCClient
	id     string
	// Resource management
	state chatState
	// For returning our resources, send struct{}{}.
	done chan<- struct{}
}

type chatState int8

const (
	running chatState = iota
	requestTearDown
	stopped
)

func (s *frescoSession) Init(ctx context.Context, id string) error {
	resp, err := s.client.Init(ctx, &proto.SessionCtx{SessionID: id})
	if err != nil {
		s.TearDown()
		return err
	}
	if resp.Status != pbJob.CmdResult_SUCCESS {
		s.TearDown()
		return fmt.Errorf("could not init session: %s", resp.Msg)
	}
	// Associate session id with context for whole session
	md := metadata.Pairs("session-id", id)
	ctx = metadata.NewContext(ctx, md)

	s.ctx = ctx
	s.id = id
	s.state = running

	glog.V(2).Infof("[%s] New session initiated", id)

	return nil
}

func (s *frescoSession) ID() string {
	return s.id
}

func (s *frescoSession) NextCmd(in *pbJob.SMCCmd) (out *pbJob.CmdResult, more bool) {
	defer s.condFreeResources()
	more = false

	if err := s.validateSession(in); err != nil {
		out, more = reportError(ErrSessionID)
		s.state = requestTearDown
		return
	}
	glog.V(3).Infof("In:  session->%v, ID->%d", in.SessionID, in.SmcPeerID)

	// Forward command to Fresco and evaluate result to manage active chat.
	out, err := s.client.NextCmd(s.ctx, in)
	if err != nil {
		out, more = reportError(err)
		s.state = requestTearDown
		return
	}

	switch {
	case pbJob.CmdResult_SUCCESS_DONE == out.Status:
		// Successful end of session
		more = false
		s.state = requestTearDown

	case pbJob.CmdResult_SUCCESS == out.Status,
		(pbJob.CmdResult_SEVERE_ERROR_CLASSES & out.Status) == 0:
		more = true

	default:
		// We're done with that session or a irreversible error occurred
		more = false
		s.state = requestTearDown
	}
	glog.V(3).Infof("Out: more->%v, res->%v, msg->%s", more, out.Status, out.Msg)
	return
}

func (s *frescoSession) TearDown() {
	if s.state != stopped {
		s.state = requestTearDown
		s.condFreeResources()

	} else {
		glog.V(2).Infof("No tear-down, already invoked.")
	}
}

func (s *frescoSession) condFreeResources() {
	if s.state == requestTearDown {
		s.state = stopped
		// TODO: find better solution
		if s.client != nil && s.id != "" {
			s.client.TearDown(context.Background(), &proto.SessionCtx{SessionID: s.id})
			glog.V(1).Infof("[%s] Teared-down SMC backend successfully", s.id)
		}
		s.conn.Close()
		// Invalidate session and release worker resource.
		s.done <- struct{}{}
	}
}

func (s *frescoSession) validateSession(in *pbJob.SMCCmd) error {
	if s.id != in.SessionID {
		return ErrSessionID
	}
	return nil
}

func (m *frescoSession) Err() error {
	return fmt.Errorf("not implemented")
}
