#!/bin/bash
# Obtain ip address of the nodes --> just used to fire in config to the gRPC server
# Let's use eth0 for receiving the config
node1=$(docker inspect --format '{{ .NetworkSettings.Networks.goisis_testnet1.IPAddress }}' node1)
node2=$(docker inspect --format '{{ .NetworkSettings.Networks.goisis_testnet1.IPAddress }}' node2)
node3=$(docker inspect --format '{{ .NetworkSettings.Networks.goisis_testnet2.IPAddress }}' node3)
go run tests/adjacency_bring_up.go $node1 $node2 $node3
