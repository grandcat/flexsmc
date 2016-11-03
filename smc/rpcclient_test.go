package smc

import (
	"context"
	"log"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	pbJob "github.com/grandcat/flexsmc/proto/job"
	proto "github.com/grandcat/flexsmc/proto/smc"
)

// func Test_connect(t *testing.T) {
// 	type args struct {
// 		socket string
// 		id     int32
// 	}
// 	tests := []struct {
// 		name string
// 		args args
// 	}{
// 		{
// 			name: "p1",
// 			args: args{
// 				socket: "unix:///tmp/grpc1.sock",
// 				id:     1,
// 			},
// 		},
// 		{
// 			name: "p2",
// 			args: args{
// 				socket: "unix:///tmp/grpc2.sock",
// 				id:     2,
// 			},
// 		},
// 		{
// 			name: "p3",
// 			args: args{
// 				socket: "unix:///tmp/grpc3.sock",
// 				id:     3,
// 			},
// 		},
// 	}
// 	for _, tt := range tests {
// 		connect(tt.args.socket, tt.args.id)
// 	}
// }

func Test_ParallelConnect(t *testing.T) {
	type args struct {
		socket string
		id     int32
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "p1",
			args: args{
				socket: "unix:///tmp/grpc1.sock",
				id:     1,
			},
		},
		{
			name: "p2",
			args: args{
				socket: "unix:///tmp/grpc2.sock",
				id:     2,
			},
		},
		{
			name: "p3",
			args: args{
				socket: "unix:///tmp/grpc3.sock",
				id:     3,
			},
		},
	}

	for _, tc := range tests {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fullSMCRound(tc.args.socket, tc.args.id)
		})
	}
}

func fullSMCRound(socket string, id int32) error {
	cc, err := DialSocket(socket)
	if err != nil {
		return err
	}
	defer cc.Close()
	c := proto.NewSMCClient(cc)

	ctx, _ := context.WithTimeout(context.Background(), time.Millisecond*2200)
	// 1. Init
	r, err := c.Init(ctx, &proto.SessionCtx{SessionID: "123456789"})
	if err != nil {
		log.Printf("could not SMC Init: %v", err)
		return err
	}
	log.Printf("Init: %s", r)
	// 2. Prepare (and second prepare with one missing participant)
	md := metadata.Pairs("session-id", "123456789")
	ctx = metadata.NewContext(ctx, md)

	for i := 0; i < 2; i++ {
		m := &pbJob.SMCCmd{
			SmcPeerID: id,
			Payload: &pbJob.SMCCmd_Prepare{Prepare: &pbJob.PreparePhase{
				Participants: []*pbJob.PreparePhase_Participant{
					&pbJob.PreparePhase_Participant{
						SmcPeerID: 1,
						Endpoint:  "[::1]:10001",
					},
					&pbJob.PreparePhase_Participant{
						SmcPeerID: 2,
						Endpoint:  "[::1]:10002",
					},
					&pbJob.PreparePhase_Participant{
						SmcPeerID: 3,
						Endpoint:  "[::1]:10003",
					},
					&pbJob.PreparePhase_Participant{
						SmcPeerID: 4,
						Endpoint:  "[::1]:10004",
					},
				},
			}},
		}
		// Simulate a situation removing one node previously available
		if i == 1 {
			pl := m.Payload.(*pbJob.SMCCmd_Prepare).Prepare.Participants
			pl = pl[:len(pl)-1]
			m.Payload.(*pbJob.SMCCmd_Prepare).Prepare.Participants = pl
		}

		r, err = c.NextCmd(ctx, m)
		if err != nil {
			log.Fatalf("cmd for next phase failed: %v", err)
		}
		log.Printf("NextCmd: %s", r)
	}

	// 3. Start session
	m := &pbJob.SMCCmd{
		SmcPeerID: id,
		Payload:   &pbJob.SMCCmd_Session{Session: &pbJob.SessionPhase{}},
	}
	r, err = c.NextCmd(ctx, m)
	if err != nil {
		log.Fatalf("cmd for next phase failed: %v", err)
	}
	log.Printf("NextCmd Session: %s", r)

	// 4. Tear-down
	// Use new context as it must be done also for a stream abort.
	r, _ = c.TearDown(context.Background(), &proto.SessionCtx{SessionID: "123456789"})
	log.Printf("Teardown Session: %v", r)

	return nil
}
