#!/bin/bash
node1=$(docker inspect --format '{{ .NetworkSettings.Networks.topologies_testnet1.IPAddress }}' node1)
node2=$(docker inspect --format '{{ .NetworkSettings.Networks.topologies_testnet1.IPAddress }}' node2)
node3=$(docker inspect --format '{{ .NetworkSettings.Networks.topologies_testnet2.IPAddress }}' node3)
echo "node1: " $node1
echo "node2: " $node2
echo "node3: " $node3
