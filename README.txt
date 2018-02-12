Project
- Implement IS-IS between using containers to represent IS-IS nodes
- Send IS-IS packets inside raw ethernet frames between the containers
- Send configuration in via gRPC
- Docker-compose to bring up the topology
- Docker containers are connected initially via the docker bridge so all containers are reachable  i.e. one big LAN by default
- Can use a bunch of custom defined networks and have nodes members of various networks to create a topology
which will be learned dynamically via IS-IS, handling container failures etc. To start we'll use 3 nodes and 2 networks, where one node is an intermediate hop between the other two:

node1 <----> node2 <----> node3

Node 1 and 3 are members of two different networks and node 2 is a member of both with two virtual ethernet interfaces. The test client
node which pushes the config in is a member of both networks so it can reach all nodes.

- Docker-machine settings needed for this to work (without this the ethernet frames get dropped by the docker linux bridge) --> see config_docker_machine.sh

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
- The adjacency test configures the SIDs on both nodes via gRPC, which causes them to start flooding
the docker bridge with the multicast mac address and establish adjacencies with any other containers
running the is-is program
- Adjacency establishment for nodes with multiple interfaces

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
    - Given the above LSP DB there is actually only path for each prefix, will need a more complex topology in order to 
    really test the SPF algorithm is working
    - Something like:
    
    node1 -- node2 -- node3    
      |                 |
       -----------------
    
    In that case there will be a 2 hop path and a 1 hop path in order to reach testnet2 from node1, so SPF should prefer
    the one hop path.

- Psuedonode support and DIS election process
- Handle level hierarchy
- Detect adjacency failures (interface flaps etc.)
- Add adjacency formation jitter
- Verification of PDU length
- PtoP link support
- Crypto auth
- Add checksums
- Support hostname



