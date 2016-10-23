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
		const netSep = "unix:"
		isUnix := strings.HasPrefix(addr, netSep)
		if !isUnix {
			return nil, errors.New("unknown network type")
		}
		return net.DialTimeout(addr[:len(netSep)-1], addr[len(netSep):], timeout)
	}

	conn, err := grpc.Dial(serverAddrSock, grpc.WithDialer(dialSocket), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := proto.NewSMCClient(conn)

	// Session metadata as header
	md := metadata.Pairs("session-id", "123456789")
	ctx := metadata.NewContext(context.Background(), md)
	// 1. Init
	r, err := c.Init(ctx, &proto.SessionCtx{SessionID: 12345})
	if err != nil {
		log.Fatalf("could not SMC Init: %v", err)
	}
	log.Printf("Init: %s", r)
	// 2. Prepare
	m := &pbJob.PreparePhase{
		Participants: []*pbJob.PreparePhase_Participant{
			&pbJob.PreparePhase_Participant{
				Addr: "addr1",
			},
			&pbJob.PreparePhase_Participant{
				Addr: "addr2",
			},
		},
	}
	r, err = c.DoPrepare(ctx, m)
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("DoPrepare: %s", r)
}
