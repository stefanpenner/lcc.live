#!/bin/sh

# TODO: let's do this with the future amazing build tool
CGO_ENABLED=0 go build -ldflags "-s -w" -x -v -o lcc.live .
ls -alh lcc.live
