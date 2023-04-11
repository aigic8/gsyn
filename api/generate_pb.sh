#!/bin/sh

set -e

protoc -I=. --go_out=./pb ./pb/protos/file.proto ./pb/protos/dir.proto ./pb/protos/space.proto ./pb/protos/global.proto
mv pb/github.com/aigic8/gsyn/api/pb/*.pb.go ./pb
rm -rf pb/github.com