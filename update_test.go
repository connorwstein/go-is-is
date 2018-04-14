package main

import (
    "testing"
    "net"
    "github.com/golang/glog"
    "encoding/binary"
)

func TestUpdateLocalLspGen(t *testing.T) {
    initConfig()
    cfg.sid = "1111.1111.1112"
    adj := Adjacency{state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}}
    initInterfaces()
    cfg.interfaces[0].adj = &adj
    UpdateDBInit()
    GenerateLocalLsp()

    // Check whether it is in the LspDB 
    testLspID := [8]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12, 0x00, 0x00}
    tmp := AvlSearch(UpdateDB.Root, binary.BigEndian.Uint64(testLspID[:]))
    lsp := tmp.(*IsisLsp)
    glog.V(2).Infof("%v", cfg.interfaces[0].lspFloodStates[0]) 
    glog.V(2).Infof("%v", lsp) 
    // SRM Flag should be set on eth0
    if ! cfg.interfaces[0].lspFloodStates[binary.BigEndian.Uint64(testLspID[:])].SRM {
        t.Fail()
    }
}

func TestReachTLV(t *testing.T) {
    numRoutesPerInterface := 2
    numInterfaces := 2
    interfaces := make([]*Intf, numInterfaces)  
    
    // Only need the routes attached to the interfaces
    for i := 0; i < numInterfaces; i++ {
        interfaces[i] = &Intf{}
        interfaces[i].routes = make([]*net.IPNet, numRoutesPerInterface)
        for j := 0; j < numRoutesPerInterface; j++ {
            interfaces[i].routes[j] = &net.IPNet{IP: net.IP{172, byte(j), 0, 0}, Mask: net.IPMask{0xff, 0xff, 0x00, 0x00}}
        } 
    }
    tlv := getIPReachTLV(interfaces)
    glog.V(2).Infof("%v", tlv)
    // 12 bytes per entry in the TLV
    if int(tlv.tlv_type) != 128 || int(tlv.tlv_length) != numRoutesPerInterface*numInterfaces*12 {
        t.Fail()
    }
}

func TestNeighborTLV(t *testing.T) {
    // TODO:
}

