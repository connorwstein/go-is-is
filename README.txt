Project goal
- Implement IS-IS between two docker containers
    - Need to figure out a way to send raw ethernet frames between two containers, then I can put the IS-IS PDUs
    in there
- Each container represents an IS-IS node
- Send configuration in via gRPC
- Docker-compose to bring up the topology
- Docker containers are connected initially via the docker bridge so all containers are reachable  i.e. one big LAN by default
- How to set up p2p links ? Use a user-defined docker network?
- Step 1: Adjacency and hello LSPs
    - Requires at least two goroutines one for sending hellos and one for receiving




