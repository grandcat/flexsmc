package pipeline

import (
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/orchestration/worker"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

// ContinousChanID creates a new participants map setting a node's channel ID from a continous
// integer space starting at 1.
// Why? Necessary fix for underlying FRESCO issue: https://github.com/aicis/fresco/issues/2
type ContinousChanID struct{}

func (c *ContinousChanID) Process(task *pbJob.SMCTask, inOut *worker.JobInstruction) error {
	participants := make(map[directory.ChannelID]*directory.PeerInfo, len(inOut.Participants))

	var cid directory.ChannelID = 1
	for _, p := range inOut.Participants {
		participants[cid] = p
		cid++
	}
	inOut.Participants = participants

	return nil
}
