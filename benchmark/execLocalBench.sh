#!/bin/bash
go test -v -bench=. -benchtime 10s -args -interface=wlp4s0 -stats_id=num_3_bla_123 -stats_granularity=1
