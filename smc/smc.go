package smc

import (
	"errors"

	pbJob "github.com/grandcat/flexsmc/proto/job"
	"golang.org/x/net/context"
)

var (
	ErrSessionID = errors.New("altering session not allowed here")
)

// Connector opens a Session with a SMC backend to control and track its computation.
type Connector interface {
	// RequestSession blocks until resources allow for a new Session.
	// The context can abort an ongoing request. This will cause an error.
	RequestSession(ctx context.Context) (Session, error)
	// ResetAll tears down any active sessions and cleans up.
	// Call it before requesting the first session if there are possibly any open
	// sessions due to a previous connection abort, for instance.
	ResetAll(ctx context.Context) error
}

type Session interface {
	Init(ctx context.Context, id string) error
	// ID of the current session.
	ID() string
	// NextCmd evaluates the input command, forwards it to the SMC backend and sends back
	// the result of the command evaluation.
	// It blocks until the request is processed or the context is done.
	NextCmd(in *pbJob.SMCCmd) (out *pbJob.CmdResult, more bool)
	// TearDown finishes a session and frees occupied resources. This should be called
	// if the originator is not interested anymore to keep the SMC reservation alive.
	TearDown()
	// Err returns a non-nil if an error occurred during any phase.
	Err() error
}

// DefaultSMCConnector set to a fake connector. For debugging.
// var DefaultSMCConnector = newSMCConnectorMock

// DefaultSMCConnector set to Fresco connector. Requires flexsmc-fresco to be up.
var DefaultSMCConnector = newFrescoConnector
