#!/bin/bash
#######################################################################
## This script starts a FlexSMC node and uses the host-specific
## configuration file `flex-host.sh` therefore if available.
## Depending on the configuration, it starts either a Gateway node
## or a regular node (also called sensor node).
#######################################################################

scriptPath=$(dirname $0)
# Include the host-specific configuration.
if [ -f "${scriptPath}/../flex-host.sh" ]; then
	. ${scriptPath}/../flex-host.sh
else
	>&2 echo "No flex-host.sh found! Using defaults."
fi

# Set defaults or replace them by configuration.
ID="${FLEX_ID:-1}"
gwRole="${FLEX_ROLE_GW:-0}"
eth="${FLEX_IFACE:-}"
logLev="${DEBUG_LEVEL:-1}"
# Sensor node specific settings.
smcSocket="${FLEX_SMCSOCK:-unix:///tmp/grpc-flexsmc1.sock}"
enPairing=1

# Assembly of options passed to flexsmc executable.
opts=""
# Set node certificates.
cert_dir="certs"
opts+=" -key_file ${cert_dir}/key_${ID}.pem -cert_file ${cert_dir}/cert_${ID}.pem"

# Set custom interface.
if [[ ! -z "${eth// }" ]]; then
	opts+=" -interface ${eth// }"
	echo "Custom inteface: ${eth// }"
fi

if [ $gwRole -eq 1 ]; then
	## Gateway specific configuration.
	opts+=" -gateway"
	echo "Node is gateway"
else
	## Sensor node specific configuration.
	opts+=" -pairing=${enPairing} -smcsocket ${smcSocket}"
fi

# Logging
logOpts="-log_dir logs -v ${logLev} -alsologtostderr"

# Aggregated params
FLEX_ARGS="${opts} ${logOpts}"

echo "Execute: ${scriptPath}/flexsmc ${FLEX_ARGS}"
${scriptPath}/flexsmc ${FLEX_ARGS}
