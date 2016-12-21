package benchmark

import (
	"flag"
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

var server *testNode

func init() {
	flag.Parse()

	// Start GW server.
	server = newTestNode()
	server.startGateway()
	time.Sleep(time.Second * 12)
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

func frescoPing(b *testing.B, tn *testNode, task *pbJob.SMCTask, timeout time.Duration) {
	jCtx, jCancel := context.WithTimeout(context.Background(), timeout)

	// b.ResetTimer()
	start := statistics.StartTrack()
	res, err := tn.orch.Request(jCtx, task)
	statistics.G(0).End(res, start, task.Aggregator.String())
	// b.StopTimer()
	if err != nil {
		b.Logf("Request failed: %v", err)
	}
	if res != nil {
		b.Logf("SMC result: %f", res.Res)
	}

	jCancel()
}

func BenchmarkAppendFloat(b *testing.B) {
	benchmarks := []struct {
		name string
		task *pbJob.SMCTask
	}{
		{"GroupDebug", &pbJob.SMCTask{Set: "debuggroup", Aggregator: pbJob.Aggregator_DBG_PINGPONG}},
		{"Group2", &pbJob.SMCTask{Set: "dummygroup2", Aggregator: pbJob.Aggregator_SUM}},
	}
	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				frescoPing(b, server, bm.task, time.Second*7)
				b.Logf("Iter %d", i)

				// Give nodes some ms to recover for next job round.
				// b.StopTimer()
				statistics.GracefulFlush()
				time.Sleep(time.Millisecond * 30)
				// b.StartTimer()
			}
		})
	}
}
