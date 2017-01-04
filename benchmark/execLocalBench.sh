#!/bin/bash
go test -v -bench=. -benchtime 4s -args -interface=wlp4s0 -stats_granularity=1 ${*}
