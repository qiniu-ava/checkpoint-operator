#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

vendor/k8s.io/code-generator/generate-groups.sh \
	"deepcopy,client,informer,lister" \
	github.com/qiniu-ava/snapshot-operator/pkg/client \
	github.com/qiniu-ava/snapshot-operator/pkg/apis \
	ava:v1alpha1 \
	--go-header-file "./tmp/codegen/boilerplate.go.txt"
