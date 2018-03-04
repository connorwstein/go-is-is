This project is an implementation of a subset of the IS-IS protocol. There may be slight deviations from the actual RFC as it is more of an exploration of golang rather than anything like a piece of production software.

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

DONE:
- The adjacency test configures the SIDs on both nodes via gRPC, which causes them to start flooding the docker bridge with the multicast mac address and establish adjacencies with any other containers running the is-is program
- Adjacency establishment for nodes with multiple interfaces
- Formation of the LSP with no reachability TLV

TODO:
- LSP exchange to build a LSP database on each node
    - For this topology the final result on each node should be

        1111.00-00  // Node 1
            TLV 128 Internal Reachability
                Prefix testnet1 metric 10
        1112.00-00 // Node 2 
            TLV 128 Internal Reachability
                Prefix testnet1 metric 10
                Prefix testnet2 metric 10 
        1113.00-00 // Node 3
            TLV 128 Internal Reachability
                Prefix testnet2 metric 10
            
- Run SPF on the LSP database
    - Given the above LSP DB there is actually only path for each prefix, will need a more complex topology in order to really test the SPF algorithm is working
    - Something like:
    
    node1 -- node2 -- node3    
      |                 |
       -----------------
    
    In that case there will be a 2 hop path and a 1 hop path in order to reach testnet2 from node1, so SPF should prefer
    the one hop path.
- Psuedonode support and DIS election process
- Detect adjacency failures (interface flaps etc.)
- Add adjacency formation jitter
- Verification of PDU length
- Crypto auth
- Add checksums
- Support hostname
