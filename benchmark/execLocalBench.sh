#!/bin/bash
# benchTasks="SimpleStaticSum|FlexPing|E2EFrescoPing|VectorSumSeparateLinking"
benchTasks="VectorSumSeparateLinking"
go test -v -bench=${benchTasks} -benchtime 4s -args -interface=wlp4s0 -stats_granularity=1 ${*}
