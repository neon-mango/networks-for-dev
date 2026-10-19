#!/usr/bin/env bash

# Builds dnsd and runs it for manual testing: a docker container with
# socat forwards its port 53 to the server, the checks are done by
# hand in the container shell. Exiting the shell stops the server,
# the EXIT trap cleans everything up on any error as well.
#
# Usage:
#   ./manual-test.sh [listen] [upstream]
#
# Defaults:
#   listen   127.0.0.1:12345
#   upstream 1.1.1.1:53

set -euo pipefail
set -x

script_path=$(readlink -e "${BASH_SOURCE[0]}")
script_dir=$(dirname "$script_path")
cd "$script_dir"

image="ubuntu:26.04"
listen="127.0.0.1:12345"
upstream="${1:-1.1.1.1:53}"
port="${listen##*:}"

server_pid=
container_name="dnsd-manual-$$"

cleanup() {
	docker rm -f "${container_name}" 2>/dev/null || true
	kill "$server_pid" 2>/dev/null || true
	wait "$server_pid" 2>/dev/null || true
}
trap cleanup EXIT

go build -o dnsd ./cmd/dnsd

./dnsd -listen "$listen" -upstream "$upstream" &
server_pid=$!

sleep 0.5
if ! kill -0 "$server_pid" 2>/dev/null; then
	echo "The server exited immediately" >&2
	exit 1
fi

docker run --rm -d --network=host --name "$container_name" "$image" sleep infinity > /dev/null
docker exec "$container_name" sh -ec \
	"apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends dnsutils iputils-ping wget socat"

# glibc does not accept a non-standard port in resolv.conf, socat
# forwards the standard port 53 to the server.
docker exec -d "$container_name" socat UDP4-LISTEN:53,bind=127.0.0.1,fork,reuseaddr "UDP4:127.0.0.1:${port}"
docker exec -d "$container_name" socat TCP4-LISTEN:53,bind=127.0.0.1,fork,reuseaddr "UDP4:127.0.0.1:${port}"
docker exec "$container_name" sh -c \
	"printf 'nameserver 127.0.0.1\noptions timeout:2 attempts:3\n' > /etc/resolv.conf"

cat <<EOF
Выход из shell остановит сервер и удалит контейнер.
Можете выполнить nslookup ya.ru, nslookup google.com
EOF

docker exec -it "$container_name" bash
