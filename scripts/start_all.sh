#!/bin/bash
# Wipe existing
docker exec node1 /bin/bash -c "pkill -f go-is-is && echo '' > /tmp/logs"
docker exec node2 /bin/bash -c "pkill -f go-is-is && echo '' > /tmp/logs"
docker exec node3 /bin/bash -c "pkill -f go-is-is && echo '' > /tmp/logs"
# Start an is-is node in all containers
docker exec -d node1 /bin/bash -c "/root/go/src/github.com/connorwstein/go-is-is/scripts/run.sh &> /tmp/logs"
docker exec -d node2 /bin/bash -c "/root/go/src/github.com/connorwstein/go-is-is/scripts/run.sh &> /tmp/logs"
docker exec -d node3 /bin/bash -c "/root/go/src/github.com/connorwstein/go-is-is/scripts/run.sh &> /tmp/logs"

