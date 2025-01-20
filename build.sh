#!/bin/sh

# TODO: let's do this with the future amazing build tool
go build -ldflags "-s -w" -x -v -o lcc.live .
ls -alh lcc.live
