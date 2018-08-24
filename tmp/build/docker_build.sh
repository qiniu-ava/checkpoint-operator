#!/usr/bin/env bash

REG="reg.qiniu.com/ava-os"
TAG="latest"

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
	echo "building container snapshot-operator..."
	docker build -t $REG/snapshot-operator:$TAG -f tmp/build/operator.Dockerfile .
	if $PUSH; then
		docker push $REG/snapshot-operator:$TAG
	fi
fi

if $WORKER; then
	echo "building container snapshot-worker..."
	docker build -t $REG/snapshot-worker:$TAG -f tmp/build/worker.Dockerfile .
	if $PUSH; then
		docker push $REG/snapshot-worker:$TAG
	fi
fi
