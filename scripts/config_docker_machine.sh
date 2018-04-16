#!/bin/bash
# Run this as root in the docker-machine vm
# i.e. docker-machine ssh, sudo su, ./configure_docker-machine
echo 0 > /proc/sys/net/bridge/bridge-nf-call-iptables 
find /sys/devices/virtual/net/ -regex '/sys/devices/virtual/net/br-.*/bridge/multicast_snooping' -exec sh -c 'echo 0 > {}' \;

