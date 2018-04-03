package main 

import (
    "testing"
    "fmt"
    "net"
    "flag"
)

func turnFlagsOn() {
    // Need this so PrintUpdateDB shows up during test run output
    flag.Set("alsologtostderr", "true")
    flag.Set("v", "2")
    flag.Parse()
}

func TestDecisionSPF(t *testing.T) {
    // Build a sample update database then apply SPF
    // Required for this: interfaces with routes and adjacencies to build the reachability and neighbor TLVs
    // Adjacencies need a neighbor system id
    // TODO: maybe this topology can be extracted automatically from a docker-compose file which could also be used for scale tests
    // will need a way to generate scale topologies eventually 
    initConfig()
    UpdateDBInit()
    fmt.Println(UpdateDB)

    // R1 
    r1Interfaces := make([]*Intf, 1)
    r1Interfaces[0] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12}}}
    r1Interfaces[0].routes = make([]*net.IPNet, 1)
    r1Interfaces[0].routes[0] = &net.IPNet{IP: net.IP{172, 20, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
    r1sid := "1111.1111.1111"
    r1neighborTLV := getNeighborTLV(r1Interfaces)
    r1reachTLV := getIPReachTLV(r1Interfaces)
    r1lsp := buildEmptyLSP(1, r1sid) 
    r1reachTLV.next_tlv = r1neighborTLV
    r1lsp.CoreLsp.FirstTlv = r1reachTLV
    UpdateDB.Root = AvlInsert(UpdateDB.Root, SystemIDToKey(r1sid), r1lsp, false)

    // R2
    r2Interfaces := make([]*Intf, 2)
    r2Interfaces[0] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}}}
    r2Interfaces[1] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x13}}}
    r2Interfaces[0].routes = make([]*net.IPNet, 1)
    r2Interfaces[1].routes = make([]*net.IPNet, 1)
    r2Interfaces[0].routes[0] = &net.IPNet{IP: net.IP{172, 20, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
    r2Interfaces[1].routes[0] = &net.IPNet{IP: net.IP{172, 19, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
    r2sid := "1111.1111.1112"
    r2neighborTLV := getNeighborTLV(r2Interfaces)
    r2reachTLV := getIPReachTLV(r2Interfaces)
    r2lsp := buildEmptyLSP(1, r2sid) 
    r2reachTLV.next_tlv = r2neighborTLV
    r2lsp.CoreLsp.FirstTlv = r2reachTLV
    UpdateDB.Root = AvlInsert(UpdateDB.Root, SystemIDToKey(r2sid), r2lsp, false)

    // R3
    r3Interfaces := make([]*Intf, 1)
    r3Interfaces[0] = &Intf{adj: &Adjacency{metric: 10, state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12}}}
    r3Interfaces[0].routes = make([]*net.IPNet, 1)
    r3Interfaces[0].routes[0] = &net.IPNet{IP: net.IP{172, 19, 0, 0}, Mask: net.IPMask{0xff, 0xff, 0, 0}}
    r3sid := "1111.1111.1113"
    r3neighborTLV := getNeighborTLV(r3Interfaces)
    r3reachTLV := getIPReachTLV(r3Interfaces)
    r3lsp := buildEmptyLSP(1, r3sid) 
    r3reachTLV.next_tlv = r3neighborTLV
    r3lsp.CoreLsp.FirstTlv = r3reachTLV
    UpdateDB.Root = AvlInsert(UpdateDB.Root, SystemIDToKey(r3sid), r3lsp, false)


    PrintUpdateDB(UpdateDB.Root)
    
    // Lets compute SPF from the perspective of R1
    turnFlagsOn()
    DecisionDBInit()
    computeSPF(UpdateDB, DecisionDB, r1sid, r1Interfaces)
    // Now print the decision DB and inspect it
}
