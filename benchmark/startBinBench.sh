#!/bin/bash

scriptPath=$(dirname $0)
# Include the host-specific configuration.
if [ -f "${scriptPath}/../../flex-host.sh" ]; then
	. ${scriptPath}/../../flex-host.sh
else
	>&2 echo "No flex-host.sh found! Using defaults."
fi

# Set defaults for node configuration.
tmpID=${FLEX_ID:-1}
ID="$(printf "%02d" $tmpID)"  # Zero pad for ascending sorting of peers.
eth="${FLEX_IFACE:-}"

# Test and benchmark parameters
reqPeers="${1:-0}"
benchInfo="${2:-0}"
benchTime="${BENCH_TIME:-10s}"
# Benchmark tests
benchTasks="${3:-.}"

# Assembly options passed to flexsmc executable.
FLEX_ARGS=""
# Set node certificates.
cert_dir="../certs"
FLEX_ARGS+=" -key_file ${cert_dir}/key_${ID}.pem -cert_file ${cert_dir}/cert_${ID}.pem"

# Set custom interface.
if [[ ! -z "${eth// }" ]]; then
	FLEX_ARGS+=" -interface ${eth// }"
	echo "Custom inteface: ${eth// }"
fi

cmd="binBench -test.bench=${benchTasks} -test.v=1 -test.benchtime ${benchTime} ${FLEX_ARGS} -bench_id=${benchInfo} -stats_granularity=1 -req_nodes=${reqPeers} -v 2"
echo ${scriptPath}/$cmd
${scriptPath}/${cmd}
