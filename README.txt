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
    - 3 way handshake for IIH: R1 sends a broadcast mac IIH frame and marks the adjacency as NEW, R2 receives this and slaps the senders mac in TLV IS-Neighbor and sends it back, then R1 receives this IIH with his own mac in it and sends back a third IIH with the senders mac (R2) in the TLV, then marks the adjacency as UP.

- Docker-machine settings needed for this work:

echo 0 > /proc/sys/net/bridge/bridge-nf-call-iptables
echo 0 > /sys/devices/virtual/net/br-c289e1f46025/bridge/multicast_snooping



