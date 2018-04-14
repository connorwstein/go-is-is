#!/bin/bash
# Wipe existing
docker exec node1 /bin/bash -c "pkill -f main && echo '' > /tmp/logs"
docker exec node2 /bin/bash -c "pkill -f main && echo '' > /tmp/logs"
docker exec node3 /bin/bash -c "pkill -f main && echo '' > /tmp/logs"
# Start an is-is node in all containers
docker exec -d node1 /bin/bash -c "/opt/go-is-is/scripts/run.sh &> /tmp/logs"
docker exec -d node2 /bin/bash -c "/opt/go-is-is/scripts/run.sh &> /tmp/logs"
docker exec -d node3 /bin/bash -c "/opt/go-is-is/scripts/run.sh &> /tmp/logs"

