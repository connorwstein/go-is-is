#!/bin/bash
#docker run -v $(pwd):/opt/go-is-is is-is-node /opt/go-is-is/build.sh
docker build -t is-is-node .
docker run -v "$(pwd)":/opt/go-is-is is-is-node /bin/bash -c "cd /opt/go-is-is && protoc -I config/ --go_out=plugins=grpc:config/ config/config.proto && go build -o main"
