#!/bin/bash
if [[ $# != 1 ]]; then
	echo "Specify the node in the topology you want an IP for"
	exit 1
fi
docker inspect node$1 | sed -n 's/\s*"IPAddress": "\(.*\)",/\1/p'  | tail -1
