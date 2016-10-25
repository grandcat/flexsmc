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

func connect() {
	dialSocket := func(addr string, timeout time.Duration) (net.Conn, error) {
		const netSep = "unix"
		isUnixSock := strings.HasPrefix(addr, netSep)
		s := strings.Index(addr, ":")
		if !isUnixSock || s < 0 {
			return nil, errors.New("unknown network type")
		}
		return net.DialTimeout(addr[:s], addr[s+1:], timeout)
	}

	conn, err := grpc.Dial(serverAddrSock, grpc.WithDialer(dialSocket), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := proto.NewSMCClient(conn)

	ctx := context.Background()
	// 1. Init
	r, err := c.Init(ctx, &proto.SessionCtx{SessionID: "123456789"})
	if err != nil {
		log.Fatalf("could not SMC Init: %v", err)
	}
	log.Printf("Init: %s", r)
	// 2. Prepare
	md := metadata.Pairs("session-id", "123456789")
	ctx = metadata.NewContext(ctx, md)
	m := &pbJob.PreparePhase{
		Participants: []*pbJob.PreparePhase_Participant{
			&pbJob.PreparePhase_Participant{
				SmcPeerID: 1,
				Endpoint:  "addr1:11111",
			},
			&pbJob.PreparePhase_Participant{
				SmcPeerID: 2,
				Endpoint:  "addr2:22222",
			},
			&pbJob.PreparePhase_Participant{
				SmcPeerID: 3,
				Endpoint:  "addr3:33333",
			},
		},
	}
	r, err = c.DoPrepare(ctx, m)
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("DoPrepare: %s", r)
}
