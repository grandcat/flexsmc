package modules

import (
	"context"
	"io"
	"time"

	"github.com/golang/glog"
	proto "github.com/grandcat/flexsmc/proto"
	pbJob "github.com/grandcat/flexsmc/proto/job"
	"github.com/grandcat/flexsmc/smc"
)

// SMCAdvisor receives SMC jobs, schedules them to available resources of the SMC
// backend and manages the low-level connection handling between GW and this peer.
type SMCAdvisor struct {
	ModuleContext
	smcConn smc.Connector
}

func NewSMCAdvisor(modInfo ModuleContext, smcConnector smc.Connector) *SMCAdvisor {
	return &SMCAdvisor{
		ModuleContext: modInfo,
		smcConn:       smcConnector,
	}
}

func (s *SMCAdvisor) Start() {
	// Cleanup any old sessions due to a connection abort. Assume this instance is
	// the only one using the SMC backend.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.smcConn.ResetAll(ctx)
	if err != nil {
		// As the module observer starts after finishing this method, report this
		// error asynchroniously. Otherwise, it will dead-lock here.
		go s.reportFault(err)
		return
	}
	// Start SMC communication workers.
	go s.BlockingSpawn()
}

func (s *SMCAdvisor) BlockingSpawn() {
	for {
		if err := s.SpawnBridge(); err != nil {
			// Notify master about unhandled error.
			s.reportFault(err)
			break
		}
		time.Sleep(time.Second)
	}

}

func (s *SMCAdvisor) SpawnBridge() error {
	// Blocks until stand-by session from SMC backend is available for Reservation.
	smcSession, err := s.smcConn.RequestSession(s.context)
	if err != nil {
		glog.V(1).Infof("Reservation of SMC backend failed: %v", err)
		return err
	}
	// once a stand-by SMC instance is available, a C&C channel is established
	// to receive SMC commands.
	stream, err := s.GWConn.AwaitSMCRound(s.context)
	if err != nil {
		smcSession.TearDown()
		glog.V(1).Infof("Could not receive SMC cmds: %v", err)
		return err
	}

	s.ActiveMods.Add(1)
	go s.bridgeStreamToSMC(stream, smcSession)
	glog.V(1).Infof("Spawned new listener routine for SMC channel to GW")

	return nil
}

func (s *SMCAdvisor) bridgeStreamToSMC(stream proto.Gateway_AwaitSMCRoundClient, smcSess smc.Session) {
	defer s.ActiveMods.Done()
	defer smcSess.TearDown()

	glog.V(2).Infoln("GW <--> Me: bridge established")

	moreCmds := true
	var cntCmds uint

	for moreCmds {
		in, err := stream.Recv()
		if err == io.EOF {
			glog.Warningf("->Rcv: GW EOF")
			break
		}
		if err != nil {
			glog.Warningf("->Rcv: GW error: %v", err)
			break
		}
		glog.V(3).Infof("->Rcv: %v", in)
		// Initialize session on first interaction.
		if cntCmds == 0 {
			if err := smcSess.Init(stream.Context(), in.SessionID); err != nil {
				glog.V(1).Infof("could not init: %v", err)
				stream.Send(&pbJob.CmdResult{
					Status: pbJob.CmdResult_ABORTED,
					Msg:    "invalid session",
				})
				moreCmds = false
				break
			}
		}
		// Trigger state machine in SMC backend and send generated response.
		var resp *pbJob.CmdResult
		resp, moreCmds = smcSess.NextCmd(in)
		// Send back response
		glog.V(3).Infof("Send->: suspect more? %v", moreCmds)
		stream.Send(resp)

		cntCmds++
	}

	// A new stream is created for the next SMC round. So close this one.
	// XXX: reuse stream to save resources, but requires stream management
	stream.CloseSend()

	glog.V(2).Infof("Close chat to GW. Suspected more msgs from GW->%v", moreCmds)
}
