#!/bin/bash
# Obtain ip address of the nodes
node1=$(docker inspect --format '{{ .NetworkSettings.Networks.goisis_default.IPAddress }}' node1)
node2=$(docker inspect --format '{{ .NetworkSettings.Networks.goisis_default.IPAddress }}' node2)
go run tests/adjacency_bring_up.go $node1 $node2
