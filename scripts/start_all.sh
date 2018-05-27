#!/bin/bash
# Wipe existing
if [[ $# != 1 ]]; then
	echo "Specify the number of nodes in the topology"
	exit 1
fi
for (( i=1; i<=$1; i++ )); do 
docker exec node$i /bin/bash -c "pkill -f go-is-is && echo '' > /tmp/logs";
done 
for (( i=1; i<=$1; i++ )); do 
docker exec -d node$i /bin/bash -c "/root/go/src/github.com/connorwstein/go-is-is/go-is-is -logtostderr -v 2 &> /tmp/logs";
done 

