package aggregation

import (
	"errors"
	"log"

	"github.com/grandcat/flexsmc/orchestration/worker"
	proto "github.com/grandcat/flexsmc/proto"
	"golang.org/x/net/context"
)

type Aggregator struct {
}

func (a *Aggregator) Process(ctx context.Context, in worker.JobWatcher) (*proto.SMCResult, error) {
	// task := in.Job()
	// len(task.Participants)
loop:
	for {
		select {
		case res, ok := <-in.Result():
			if !ok {
				err := in.Err()
				log.Println(">> Aggregator: end of stream with error:", err)
				if err != nil {
					// XXX: return anonymous result only (without details about peers)?
					return nil, err
				}
				break loop
			}
			log.Printf(">> Aggregator: received %v", res.Response)

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, errors.New("not implemented")
}
