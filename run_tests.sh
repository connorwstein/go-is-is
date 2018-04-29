#!/bin/bash
# Obtain ip address of the nodes --> just used to fire in config to the gRPC server
# Let's use eth0 for receiving the config
export node1=$(docker inspect --format '{{ .NetworkSettings.Networks.topologies_testnet1.IPAddress }}' node1)
export node2=$(docker inspect --format '{{ .NetworkSettings.Networks.topologies_testnet1.IPAddress }}' node2)
export node3=$(docker inspect --format '{{ .NetworkSettings.Networks.topologies_testnet2.IPAddress }}' node3)
go test -v -coverprofile cover.out
