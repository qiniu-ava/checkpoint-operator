#!/usr/bin/env bash

if ! which docker >/dev/null; then
	echo "docker needs to be installed"
	exit 1
fi

echo "building container checkpoint-operator..."
docker build -t checkpoint-operator -f tmp/build/operator.Dockerfile .

echo "building container checkpoint-worker..."
docker build -t checkpoint-worker -f tmp/build/worker.Dockerfile .
