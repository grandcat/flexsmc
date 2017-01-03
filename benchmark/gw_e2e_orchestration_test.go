package benchmark

import (
	"flag"
	"strconv"
	"testing"
	"time"

	"context"

	"github.com/grandcat/flexsmc/benchmark/statistics"
	"github.com/grandcat/flexsmc/directory"
	"github.com/grandcat/flexsmc/node"
	"github.com/grandcat/flexsmc/orchestration"
	pbJob "github.com/grandcat/flexsmc/proto/job"
)

var (
	certFile = flag.String("cert_file", "../certs/cert_1.pem", "TLS cert file")
	keyFile  = flag.String("key_file", "../certs/key_1.pem", "TLS key file")
	iface    = flag.String("interface", "", "Network interface to use for discovery, e.g. enp3s0")
)

var (
	reqNumPeers = flag.Int("req_nodes", 0, "Required number of participating peers. 0 means any number of currently online peers.")
)

var server *testNode

func init() {
	flag.Parse()

	// Start GW server.
	server = newTestNode()
	server.startGateway()
}

type testNode struct {
	// Common test environment.
	gw   *node.Gateway
	reg  *directory.Registry
	orch orchestration.Orchestration
}

func newTestNode() *testNode {
	registry := directory.NewRegistry()
	opts := node.GWOptions{
		Options: node.Options{
			CertFile: *certFile,
			KeyFile:  *keyFile,
			Inteface: *iface,
		},
		Registry:        registry,
		AnnounceService: true,
	}
	gw := node.NewGateway(opts)

	te := &testNode{
		gw:   gw,
		reg:  registry,
		orch: orchestration.NewFIFOOrchestration(registry),
	}
	return te
}

func (tn *testNode) startGateway() {
	go tn.gw.Run()
}

// awaitPeers keeps on submitting simple probes until the pipe
// accepts the request due to the statistied amount of connected peers.
func (tn *testNode) awaitPeers(timeout time.Duration, reqNumber int32) error {
	jCtx, jCancel := context.WithTimeout(context.Background(), timeout)
	defer jCancel()

	taskOption := map[string]*pbJob.Option{
		"minNumPeers": &pbJob.Option{&pbJob.Option_Dec{reqNumber}},
		"maxNumPeers": &pbJob.Option{&pbJob.Option_Dec{reqNumber}},
	}
	task := &pbJob.SMCTask{
		Set:        "Wait_for_enough_peers",
		Aggregator: pbJob.Aggregator_DBG_PINGPONG,
		Options:    taskOption,
	}

	for jCtx.Err() == nil {
		_, err := tn.orch.Request(jCtx, task)
		if err == nil {
			return nil
		}
		time.Sleep(time.Second * 2)
	}

	return jCtx.Err()
}

func frescoPing(b *testing.B, tn *testNode, task *pbJob.SMCTask, timeout time.Duration) {
	jCtx, jCancel := context.WithTimeout(context.Background(), timeout)

	// b.ResetTimer()
	start := statistics.StartTrack()
	res, err := tn.orch.Request(jCtx, task)

	var resStr, resErr string
	if err != nil {
		resErr = err.Error()
	} else {
		resStr = strconv.FormatFloat(res.Res, 'f', 2, 64)
	}
	statistics.G(0).End(res, start, task.Aggregator.String(), resStr, resErr)
	// b.StopTimer()
	if err != nil {
		b.Logf("Request failed: %v", err)
	}
	if res != nil {
		b.Logf("SMC result: %f", res.Res)
	}

	jCancel()
}

func BenchmarkFrescoE2ESimple(b *testing.B) {
	// Wait for peers to become ready.
	taskOption := map[string]*pbJob.Option{}
	if *reqNumPeers != 0 {
		n := int32(*reqNumPeers)
		b.Logf("Waiting for %d nodes to become ready...", n)
		err := server.awaitPeers(time.Second*40, n)
		if err != nil {
			b.Fatal("Not enough peers:", err.Error())
			b.FailNow()
		}
		b.Logf("%d nodes ready, starting.", n)

		// Configure taskOption to maintain required number of peers constantly.
		taskOption["minNumPeers"] = &pbJob.Option{&pbJob.Option_Dec{n}}
		taskOption["maxNumPeers"] = &pbJob.Option{&pbJob.Option_Dec{n}}
	}
	// Run bench.
	benchmarks := []struct {
		name string
		task *pbJob.SMCTask
	}{
		{"PingPong", &pbJob.SMCTask{Set: "bench", Aggregator: pbJob.Aggregator_DBG_PINGPONG, Options: taskOption}},
		{"SingleSum", &pbJob.SMCTask{Set: "benchgroup2", Aggregator: pbJob.Aggregator_SUM, Options: taskOption}},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				frescoPing(b, server, bm.task, time.Second*7)
				b.Logf("Iter %d", i)

				// Give nodes some ms to recover for next job round.
				// b.StopTimer()
				statistics.GracefulFlush()
				time.Sleep(time.Millisecond * 50)
				// b.StartTimer()
			}
		})
		// Write buffered statistics to disk if necessary.
		statistics.GracefulFlush()
	}
}
