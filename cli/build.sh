#!/usr/bin/env bash


echo -n "Version? "
read version
export CGO_ENABLED=0
set -x -u -o pipefail
gox --output="build/agnosticv_{{.OS}}_{{.Arch}}"  -ldflags="-X 'main.Version=${version}' -X 'main.buildTime=$(date -u)' -X 'main.buildCommit=$(git rev-parse HEAD)'"
env GOOS=darwin GOARCH=arm64 go build -ldflags="-X 'main.Version=${version}' -X 'main.buildTime=$(date -u)' -X 'main.buildCommit=$(git rev-parse HEAD)'" -o build/agnosticv_darwin_arm64
