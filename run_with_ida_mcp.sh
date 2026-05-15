#!/usr/bin/env bash
set -e

WIN_HOST=$(ip route show | grep -i default | awk '{ print $3 }')

export IDA_MCP_URL="http://${WIN_HOST}:13338/sse"
export IDA_MCP_HOST_HEADER="127.0.0.1:13338"
export NO_PROXY="localhost,127.0.0.1,::1,${WIN_HOST}"
export no_proxy="$NO_PROXY"

echo "[INFO] WIN_HOST=${WIN_HOST}"
echo "[INFO] IDA_MCP_URL=${IDA_MCP_URL}"
echo "[INFO] IDA_MCP_HOST_HEADER=${IDA_MCP_HOST_HEADER}"

go run ./cmd/server
