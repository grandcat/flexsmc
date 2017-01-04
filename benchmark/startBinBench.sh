#!/bin/bash

scriptPath=$(dirname $0)
# Include the host-specific configuration.
if [ -f "${scriptPath}/../../flex-host.sh" ]; then
	. ${scriptPath}/../../flex-host.sh
else
	>&2 echo "No flex-host.sh found! Using defaults."
fi

# Set defaults for node configuration.
ID="${FLEX_ID:-1}"
eth="${FLEX_IFACE:-}"
logLev="${DEBUG_LEVEL:-1}"

# Test and benchmark parameters
reqPeers="${1:-0}"
benchTime="${BENCH_TIME:-10s}"

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

# Construct experiment ID
EXPERIMENT_ID="peers_${reqPeers}_cores_0_freq_0_netlat_0"

echo "Args: ${FLEX_ARGS} ${*}"

cmd="binBench -test.bench=. -test.v=1 -test.benchtime ${benchTime} ${FLEX_ARGS} -stats_granularity=1 -stats_id=${EXPERIMENT_ID} -req_nodes=${reqPeers}"
echo ${scriptPath}/$cmd
${scriptPath}/${cmd}
