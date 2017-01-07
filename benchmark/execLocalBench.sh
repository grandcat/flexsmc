#!/bin/bash
# benchTasks="SimpleStaticSum|E2EFresco1Ping"
benchTasks="SimpleStaticSum"
go test -v -bench=${benchTasks} -benchtime 4s -args -interface=wlp4s0 -stats_granularity=1 ${*}
