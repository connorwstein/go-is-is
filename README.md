[![Build Status](https://travis-ci.org/connorwstein/go-is-is.svg?branch=master)](https://travis-ci.org/connorwstein/go-is-is)

This project is an implementation of the major aspects of the IS-IS routing protocol, although it is by no means the whole exact RFC. The idea is given an arbitrarily complex network of containers, the go-is-is process could be run on each one of them, learn the whole topology and then install routes in the containers so containers will be reachable across networks.

For example in the topology:

node1 <----> node2 <----> node3

Say node 1 and 3 are members of two different networks and node 2 is a member of both with two virtual ethernet interfaces. After the IS-IS protocol has established neighbors, exchanged LSPs and ran SPF on the LSP database, then each node will know the full topology of the network and know the shortest path to each node. Node 1 will know that he has to go through node 2 to get to node 3. 

Node configuration and state queries are accepted through gRPC and state information can also be streamed out in the same fashion. The test client node which pushes the config in is a member of all networks so it can reach all nodes. Also since IS-IS frames are embedded directly in layer 2 packets with no layer 3 header, special settings are required for the linux kernel. See config_docker_machine.sh for details.

USAGE:
Topology bring up:
docker-compose build
docker-compose up

Build the IS-IS binary and start it on each node:
docker exec -d test_client /bin/bash -c "/opt/go-is-is/build.sh"
docker exec -d test_client /bin/bash -c "/opt/go-is-is/scripts/start_all.sh"

Run the tests:
docker exec test_client /opt/go-is-is/run_tests.sh

See all the container IPs in the topology:
docker exec test_client /bin/bash -c "go run /opt/go-is-is/scripts/get_ips.sh"

Show the running state of each node:
docker exec test_client /bin/bash -c "go run /opt/go-is-is/scripts/show_run.go <IP OF CONTAINER>"

DONE:
- Adjacency establishment for nodes with multiple interfaces
- LSP exchange to build a LSP database on each node
- Support sequence numbers of LSPs to overwrite if we get a newer sequence number
- Using the metrics in TLV 2 and TLV 128, run SPF on the LSP database. SPF runs on a graph where
nodes are IS-IS instances, adjacencies are edges and directly connected prefixes are leaf nodes.

TODO:
- More complex topology for fully testing SPF
- Actually install the routes to make all containers reachable
- Might be able to convert the structs to use byte slices for everything rather than fixed sizes
- Interface information should probably be a map not a list
- Replace sleeps with timers
- Detect adjacency failures (interface flaps etc.)
- More unit tests
- Scale tests 
- Performance tests

NICE TO HAVE:
- Psuedonode support and DIS election process
- CSNP/PSNP for database synchronization
- Add adjacency formation jitter
- Support reboot
- L2 Areas
- Verification of PDU length
- Crypto auth
- Add checksums
- Support hostname
