#!/usr/bin/env bash

REG=""
TAG=""

PUSH=false
CONTROLLER=false
WORKER=false

case $1 in
controller)
	CONTROLLER=true
	;;
worker)
	WORKER=true
	;;
all)
	CONTROLLER=true
	WORKER=true
	;;
esac

if [ $# -gt 1 ]; then
	if [ $2=="--push" ]; then
		PUSH=true
	fi
fi

if $CONTROLLER; then
	echo "building container checkpoint-operator..."
	docker build -t $REG/checkpoint-operator:$TAG -f tmp/build/operator.Dockerfile .
	if $PUSH; then
		docker push $REG/checkpoint-operator:$TAG
	fi
fi

if $WORKER; then
	echo "building container checkpoint-worker..."
	docker build -t $REG/checkpoint-worker:$TAG -f tmp/build/worker.Dockerfile .
	if $PUSH; then
		docker push $REG/checkpoint-worker:$TAG
	fi
fi
