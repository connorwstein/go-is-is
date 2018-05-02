#!/bin/bash
# Obtain ip address of the nodes --> just used to fire in config to the gRPC server
# Let's use eth0 for receiving the config
if [[ $# != 1 ]]; then
	echo "Specify the number of nodes in the topology"
	exit 1
fi
for (( i=1; i<= $1; i++ )); do 
    export node$i=$(docker inspect node$i | sed -n 's/\s*"IPAddress": "\(.*\)",/\1/p'  | tail -1)
done
go test -v -run TestSystemIDConfig -args -num_nodes=$1 
