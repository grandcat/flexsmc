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
	certFile = flag.String("cert_file", "../certs/cert_01.pem", "TLS cert file")
	keyFile  = flag.String("key_file", "../certs/key_01.pem", "TLS key file")
	iface    = flag.String("interface", "", "Network interface to use for discovery, e.g. enp3s0")
)

var (
	benchID     = flag.String("bench_id", "0", "ID for current benchmark. GW only")
	reqNumPeers = flag.Int("req_nodes", seqMinPeers, "Required number of participating peers.")
)

var gw *testNode

func init() {
	flag.Parse()

	// Start GW server.
	gw = newTestNode()
	gw.startGateway()
}

type testNode struct {
	// Common test environment.
	gw   *node.Gateway
	reg  *directory.Registry
	orch orchestration.Orchestration

	errCnt int
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
		"minNumPeers":    &pbJob.Option{&pbJob.Option_Dec{reqNumber}},
		"maxNumPeers":    &pbJob.Option{&pbJob.Option_Dec{reqNumber}},
		"skipSMCBackend": &pbJob.Option{&pbJob.Option_Dec{1}},
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
		Aggregator: pbJob.Aggregator_DBG_SET_CONFIG,
		Options:    taskOption,
	}

	_, err := tn.orch.Request(jCtx, task)
	return err
}

func (tn *testNode) requestSwitchLogfile(prefix string) error {
	return tn.sendDebugConfig("b.chgLog", prefix, "DBG_CONFIG_PEERS")
}

func (tn *testNode) requestUpdateSetID(expID string) error {
	return tn.sendDebugConfig("b.upExpID", expID, "DBG_CONFIG_PEERS")
}

func (tn *testNode) requestUploadBenchmarks() error {
	return tn.sendDebugConfig("b.upload", "1", "DBG_CONFIG_PEERS")
}

func (tn *testNode) submitTaskAndWait(b *testing.B, task *pbJob.SMCTask, timeout time.Duration) (string, error) {
	jCtx, jCancel := context.WithTimeout(context.Background(), timeout)
	defer jCancel()

	start := statistics.StartTrack()
	res, err := tn.orch.Request(jCtx, task)

	var resStr, resErr string
	if err != nil {
		resErr = err.Error()
	} else {
		// Result can be empty if peers do not have any useful data
		// beside the successful execution. E.g. for FlexSMC ping msgs.
		// An end user usually receives a result.
		if res != nil {
			resStr = strconv.FormatFloat(res.Res, 'f', 2, 64)
		}
	}
	statistics.G(0).End(res, start, task.Aggregator.String(), resStr, resErr)

	// Wait some time in case of error. So there is more time to recover in Fresco.
	if err != nil {
		tn.errCnt++
		fmt.Println(">>> Task error number:", tn.errCnt, "Reason", resErr)
		time.Sleep(time.Second * 15)
	}

	return resStr, err
}

func preBenchmarkRound(b *testing.B, experimentID string) {
	statistics.UpdateSetID(experimentID)
	if err := gw.requestUpdateSetID(experimentID); err != nil {
		b.Error("Could not apply new config to all peers:", err)
	}
	time.Sleep(time.Second * 3)
}

func postBenchmarkRound(b *testing.B) {
	// Write buffered statistics to disk and upload statistics.
	// - Local
	debughelper.UploadBenchmarks()
	// - Remote
	if err := gw.requestUploadBenchmarks(); err != nil {
		b.Error("Statistics upload failed:", err)
	}
}

const (
	seqMinPeers     = 7
	seqEveryNthPeer = 2
)

func doBench(b *testing.B, task *pbJob.SMCTask, info string) {
	if task.Options == nil {
		task.Options = make(map[string]*pbJob.Option)
	}
	// sortPeerIDs instructs the online filter to sort participants for
	// deterministic set of nodes if more than maxNumPeers peers are connected
	// to this GW.
	task.Options["sortPeerIDs"] = &pbJob.Option{&pbJob.Option_Dec{1}}

	// Wait for peers to become ready.
	if *reqNumPeers != 0 {
		n := int32(*reqNumPeers)
		b.Logf("Waiting for %d nodes to become ready...", n)
		err := gw.awaitPeers(time.Second*60, n)
		if err != nil {
			b.Fatal("Not enough peers:", err.Error())
			b.FailNow()
		}
		b.Logf("%d nodes ready, starting.", n)
	}

	// Create specific logfile for each benchmark.
	logPrefix := "stats." + info + "."
	statistics.SwitchLog(logPrefix)
	gw.requestSwitchLogfile(logPrefix)

	// Number of peers to evaluate.
	var numPeers []int
	for i := seqMinPeers; i <= *reqNumPeers; i += seqEveryNthPeer {
		numPeers = append(numPeers, i)
	}
	// Host configuration.
	benchmarks := []struct {
		cpu int
	}{
		{8},
	}
	// Iterate over number of participanting peers.
	for _, np := range numPeers {
		// Configure taskOption to set number of peers.
		task.Options["minNumPeers"] = &pbJob.Option{&pbJob.Option_Dec{int32(np)}}
		task.Options["maxNumPeers"] = &pbJob.Option{&pbJob.Option_Dec{int32(np)}}

		// Iterate over configurations.
		for _, bm := range benchmarks {
			// TODO: generate various experiments per test (e.g. trottle CPU, network latency, etc.)
			expID := fmt.Sprintf("bid_%s_tsk_%s_peers_%d_cpu_%d", *benchID, info, np, bm.cpu)

			preBenchmarkRound(b, expID)

			b.Run(expID, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					res, err := gw.submitTaskAndWait(b, task, time.Second*180)
					b.Logf("Iter %d: Res: %s [Err: %v]", i, res, err)

					// Give nodes some ms to recover for next job round.
					statistics.GracefulFlush()
					time.Sleep(time.Millisecond * 50)
				}
			})

			postBenchmarkRound(b)
		}
	}
}

// Tasks to benchmark

func BenchmarkE2EFrescoPing(b *testing.B) {
	task := &pbJob.SMCTask{
		Set:        "all",
		Aggregator: pbJob.Aggregator_DBG_PINGPONG,
	}

	aggregators := []struct {
		tSuffix string
		ag      pbJob.Aggregator
	}{
		{"01", pbJob.Aggregator_DBG_PINGPONG},
		{"10", pbJob.Aggregator_DBG_PINGPONG_10},
	}

	for _, conf := range aggregators {
		task.Aggregator = conf.ag
		doBench(b, task, "fresPing"+conf.tSuffix)
	}
}

// BenchmarkFlexPing
// The destination peer echoes back the request as soon as it receives the
// instruction without reaching down to the SMC backend.
//
// Test different sizes of phase concatenation.
// More precise way to determine impact of short vs long tasks with
// regard to communication overhead.
func BenchmarkFlexPing(b *testing.B) {
	task := &pbJob.SMCTask{
		Set:        "all",
		Aggregator: pbJob.Aggregator_DBG_PINGPONG,
		Options: map[string]*pbJob.Option{

			"skipSMCBackend": &pbJob.Option{&pbJob.Option_Dec{1}},
		},
	}

	aggregators := []struct {
		tSuffix string
		ag      pbJob.Aggregator
	}{
		{"001", pbJob.Aggregator_DBG_PINGPONG},
		{"010", pbJob.Aggregator_DBG_PINGPONG_10},
		// {"100", pbJob.Aggregator_DBG_PINGPONG_100},
	}

	for _, conf := range aggregators {
		task.Aggregator = conf.ag
		doBench(b, task, "flexPing"+conf.tSuffix)
	}
}

func BenchmarkSimpleStaticSumDefault(b *testing.B) {
	task := &pbJob.SMCTask{
		Set:        "all",
		Aggregator: pbJob.Aggregator_SUM,
	}
	doBench(b, task, "sisum")
}

func BenchmarkSimpleStaticSumSeparateLinking(b *testing.B) {
	task := &pbJob.SMCTask{
		Set:        "all",
		Aggregator: pbJob.Aggregator_SUM,
		Options: map[string]*pbJob.Option{
			"useLinkingPhase": &pbJob.Option{&pbJob.Option_Dec{1}},
		},
	}
	doBench(b, task, "sisumWLink")
}

func BenchmarkVectorSumSeparateLinking(b *testing.B) {
	task := &pbJob.SMCTask{
		Set:        "all",
		Aggregator: pbJob.Aggregator_SUM,
		Options: map[string]*pbJob.Option{
			"useLinkingPhase": &pbJob.Option{&pbJob.Option_Dec{1}},
		},
	}

	batchConfigs := []struct {
		maxItems  int32
		batchSize int32
	}{
		// {1000, 1},
		// {1000, 2},
		{1000, 10},
		{1000, 20},
		{1000, 50},
		{1000, 500},
		{1000, 1000},
	}

	for _, conf := range batchConfigs {
		task.MaxDataItems = conf.maxItems
		task.Options["batchOpSize"] = &pbJob.Option{&pbJob.Option_Dec{conf.batchSize}}

		doBench(b, task, fmt.Sprintf("vecsumWLink_i_%d_bt_%d", conf.maxItems, conf.batchSize))
	}
}
