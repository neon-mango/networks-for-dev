#!/usr/bin/env bash

set -xe

script_path=$(readlink -e "$BASH_SOURCE{0}")
script_dir=$(dirname "$script_path")

cd "$script_dir"

protoc --go_out=. --go_opt=module=notes --go-grpc_out=. --go-grpc_opt=module=notes grpc/notes.proto
