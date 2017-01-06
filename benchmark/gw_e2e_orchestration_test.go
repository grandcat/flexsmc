package benchmark

import (
	"flag"
	"strconv"
	"testing"
	"time"

	"context"

	"fmt"

	"github.com/grandcat/flexsmc/benchmark/debughelper"
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
	benchID     = flag.String("bench_id", "0", "ID for current benchmark. GW only")
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
	// Enable debug mode for statistics and custom task settings.
	debughelper.EnableDebugMode()

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

func (tn *testNode) sendDebugConfig(key, val, info string) error {
	jCtx, jCancel := context.WithTimeout(context.Background(), time.Second*30)
	defer jCancel()

	taskOption := map[string]*pbJob.Option{
		key: &pbJob.Option{&pbJob.Option_Str{val}},
	}
	task := &pbJob.SMCTask{
		Set:        info,
		Aggregator: pbJob.Aggregator_DBG_PINGPONG,
		Options:    taskOption,
	}

	_, err := tn.orch.Request(jCtx, task)
	return err
}

func (tn *testNode) applyConfigToOnlinePeers(expID string) error {
	return tn.sendDebugConfig("b.upExpID", expID, "DBG_CONFIG_PEERS")
}

func (tn *testNode) triggerUploadBenchmark() error {
	return tn.sendDebugConfig("b.upload", "1", "DBG_CONFIG_PEERS")
}

func frescoPing(b *testing.B, tn *testNode, task *pbJob.SMCTask, timeout time.Duration) (string, error) {
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
	jCancel()
	return resStr, err
}

func BenchmarkFrescoE2ESimple(b *testing.B) {
	// Wait for peers to become ready.
	taskOption := map[string]*pbJob.Option{}
	if *reqNumPeers != 0 {
		n := int32(*reqNumPeers)
		b.Logf("Waiting for %d nodes to become ready...", n)
		err := server.awaitPeers(time.Second*60, n)
		if err != nil {
			b.Fatal("Not enough peers:", err.Error())
			b.FailNow()
		}
		b.Logf("%d nodes ready, starting.", n)

		// Configure taskOption to maintain required number of peers constantly.
		taskOption["minNumPeers"] = &pbJob.Option{&pbJob.Option_Dec{n}}
		taskOption["maxNumPeers"] = &pbJob.Option{&pbJob.Option_Dec{n}}

		taskOption["sortPeerIDs"] = &pbJob.Option{&pbJob.Option_Dec{1}}
	}

	// Run bench.
	benchmarks := []struct {
		expName string
		task    *pbJob.SMCTask
	}{
		{"Fres1PingPong", &pbJob.SMCTask{Set: "bench", Aggregator: pbJob.Aggregator_DBG_PINGPONG, Options: taskOption}},
		// {"SingleSum", &pbJob.SMCTask{Set: "benchgroup2", Aggregator: pbJob.Aggregator_SUM, Options: taskOption}},
	}
	for _, bm := range benchmarks {
		// TODO: generate various experiments per test (e.g. trottle CPU, network latency, etc.)
		expID := fmt.Sprintf("bid_%s_job_%s_peers_%d", *benchID, bm.expName, *reqNumPeers)
		statistics.UpdateSetID(expID)
		var err error
		err = server.applyConfigToOnlinePeers(expID)
		if err != nil {
			b.Error("Could not apply new config to all peers:", err)
		}

		// TODO: run bench for each experiment.
		b.Run(expID, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				res, err := frescoPing(b, server, bm.task, time.Second*7)
				b.Logf("Iter %d: Res: %s [Err: %v]", i, res, err)

				// Give nodes some ms to recover for next job round.
				statistics.GracefulFlush()
				time.Sleep(time.Millisecond * 50)
			}
		})
		// Write buffered statistics to disk and upload statistics.
		// - Local
		debughelper.UploadBenchmarks()
		// - Remote
		err = server.triggerUploadBenchmark()
		if err != nil {
			b.Error("Statistics upload failed:", err)
		}
	}
}
