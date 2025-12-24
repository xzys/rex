#!/bin/bash

set -e

docker build -f ext/lf/Dockerfile -t r36s-lf-builder ext/lf
docker run --rm --platform linux/amd64 \
	-v go-mod-cache:/go/pkg/mod \
	-v go-build-cache:/root/.cache/go-build \
	-e GOPATH=/root/go \
	-v $(pwd)/bin:/output \
	r36s-lf-builder \
	sh -c 'go install -ldflags="-s -w" github.com/gokcehan/lf@latest && \
		cp /root/go/bin/linux_arm64/lf /output/lf'


