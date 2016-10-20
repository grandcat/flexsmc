package smc

import (
	"context"
	"log"

	pbJob "github.com/grandcat/flexsmc/proto/job"
	proto "github.com/grandcat/flexsmc/proto/smc"
	"google.golang.org/grpc"
)

const (
	serverAddr  = "localhost:50052"
	defaultName = "world"
)

func connect() {
	conn, err := grpc.Dial(serverAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := proto.NewSMCClient(conn)

	// 1. Init
	r, err := c.Init(context.Background(), &proto.SessionCtx{SessionID: 1234})
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
	r, err = c.DoPrepare(context.Background(), m)
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("DoPrepare: %s", r)
}
