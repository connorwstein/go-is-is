#!/bin/bash
protoc -I config/ --go_out=plugins=grpc:config/ config/config.proto
go build -o main
