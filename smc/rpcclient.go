package smc

import (
	"context"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	pbJob "github.com/grandcat/flexsmc/proto/job"
	proto "github.com/grandcat/flexsmc/proto/smc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	serverAddr     = "localhost:50052"
	serverAddrSock = "unix:/tmp/grpc.sock"
	defaultName    = "world"
)

func connect(socket string, id int32) {
	dialSocket := func(addr string, timeout time.Duration) (net.Conn, error) {
		const netSep = "unix"
		isUnixSock := strings.HasPrefix(addr, netSep)
		s := strings.Index(addr, ":")
		if !isUnixSock || s < 0 {
			return nil, errors.New("unknown network type")
		}
		return net.DialTimeout(addr[:s], addr[s+1:], timeout)
	}

	if socket == "" {
		socket = serverAddrSock
	}

	conn, err := grpc.Dial(socket, grpc.WithDialer(dialSocket), grpc.WithInsecure())
	if err != nil {
		log.Printf("did not connect: %v", err)
		return
	}
	defer conn.Close()

	c := proto.NewSMCClient(conn)

	ctx := context.Background()
	// 1. Init
	r, err := c.Init(ctx, &proto.SessionCtx{SessionID: "123456789"})
	if err != nil {
		log.Printf("could not SMC Init: %v", err)
		return
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

}
