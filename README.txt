This project is an implementation of the major aspects of the IS-IS protocol. There may be slight deviations from the actual RFC as it is more of an exploration of golang rather than anything like a piece of production software.

Basically the goals of this project are to create a routing protocol implementation which is:
- Actually fully tested
- Leverages golang for concurrency primitives
- Designed with telemetry in mind from the beginning 
- Extremely easy to test at scale
- Strictly interfaced with programmtically - no cli 

Traditional implementations use only a handful of threads as each one eats up something on the order of a few MB of memory. In C, having a thread per interface is out of the question because large routers can have thousands of interfaces which would mean GB of memory, but in golang it should be possible to have a goroutine per interface and not use a ton of memory. 

The nodes are represented by docker containers and a topology can be brought up with docker-compose. Node configuration and state queries are accepted through gRPC. Docker containers are placed in separate networks and then the topology is learned though IS-IS. The test topology uses 3 nodes and 2 networks:

       net1         net2 
node1 <----> node2 <----> node3

Node 1 and 3 are members of two different networks and node 2 is a member of both with two virtual ethernet interfaces. The test client node which pushes the config in is a member of both networks so it can reach all nodes. Since IS-IS frames are embedded directly in layer 2 packets with no layer 3 header, special settings are required for the linux kernel. See config_docker_machine.sh for details.

USAGE:
Topology bring up:
docker-compose build
docker-compose up

Start the IS-IS process on each node:
docker-compose exec -it node1 bash
./main -logtostderr=true
docker-compose exec -it node2 bash
./main -logtostderr=true
docker-compose exec -it node3 bash
./main -logtostderr=true

Run the tests:
docker exec -it test_client bash
./run_tests

Logging levels:
v1 --> detailed database updates
v2 --> locking and TLV data

DONE:
- The adjacency test configures the SIDs on both nodes via gRPC, which causes them to start flooding the docker bridge with the multicast mac address and establish adjacencies with any other containers running the is-is program
- Adjacency establishment for nodes with multiple interfaces
- LSP exchange to build a LSP database on each node
    - For this topology the final LSP DB on each node is
        1111.00-00  // Node 1
            TLV 128 Internal Reachability
                Prefix testnet1 metric 10
            TLV 2 IS Neighbors 
                1112.00-00
        1112.00-00 // Node 2 
            TLV 128 Internal Reachability
                Prefix testnet1 metric 10
                Prefix testnet2 metric 10 
            TLV 2 IS Neighbors
                1111.00-00
                1113.00-00
        1113.00-00 // Node 3
            TLV 128 Internal Reachability
                Prefix testnet2 metric 10
            TLV 2 IS Neighbors
                1112.00-00
- Need to support sequence numbers of LSPs to overwrite if we get a newer sequence number. Currently node2 will form an adjacency with either node 1 or 3 first, generate and send out its LSP. Then it will do that again once the second adjacency forms, but node1 and 3 will not update their databases when they get this new LSP, as they already have an LSP from node2.

TODO:
- Might be able to convert the structs to use byte slices for everything rather than fixed sizes
- Using the metrics in TLV 2 and TLV 128, run SPF on the LSP database. SPF runs on a graph where
nodes are IS-IS instances, adjacencies are edges and directly connected prefixes are leaf nodes.
All edges have a length/metric of 10. The important thing that we should see is the following on node 1:

    172.19.0.0/16 next hop is node 2 and metric is 20 

That would indicate the metric and next hop calculation is correct. Probably need more complex topology for 
fully testing SPF though.
- Replace sleeps with timers
- Detect adjacency failures (interface flaps etc.)
- Clean up naming convention
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
