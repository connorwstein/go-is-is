#!/bin/bash
tmux new-session -d -s go-is-is
tmux new-window -t go-is-is:1 -n compose
tmux select-window -t go-is-is:1
tmux split-window -h
tmux select-pane -t 0
tmux send-keys "docker-compose up" C-m
tmux select-pane -t 1
tmux send-keys "docker-machine ssh" C-m
tmux send-keys "sudo su" C-m
tmux send-keys "echo 0 > /proc/sys/net/bridge/bridge-nf-call-iptables" C-m
tmux send-keys "find /sys/devices/virtual/net/ -regex '/sys/devices/virtual/net/br-.*/bridge/multicast_snooping' -exec sh -c 'echo 0 > {}' \\\\;" C-m
tmux select-window -t go-is-is:0
tmux send-keys "docker exec -it test_client bash" C-m
tmux split-window -h
tmux send-keys "docker exec -it node1 bash" C-m
tmux split-window -v
tmux select-pane -t 0
tmux send-keys "docker exec -it node2 bash" C-m
tmux split-window -v
tmux send-keys "docker exec -it node3 bash" C-m
tmux new-window -t go-is-is:2 -n vim
tmux attach-session -t go-is-is
