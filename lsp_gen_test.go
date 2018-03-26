package main

import (
    "testing"
    "fmt"
    "sync"
    "encoding/binary"
)

func TestLocalLspGen(t *testing.T) {
    cfg = &Config{lock: sync.Mutex{}, sid: ""}
    cfg.sid = "1111.1111.1112"
    adj := Adjacency{state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}}
    initInterfaces()
    cfg.interfaces[0].adj = &adj
    LspDBInit()
    GenerateLocalLsp()

    // Check whether it is in the LspDB 
    testLspID := [8]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12, 0x00, 0x00}
    tmp := AvlSearch(LspDB.Root, binary.BigEndian.Uint64(testLspID[:]))
    lsp := tmp.(*IsisLsp)
    fmt.Println(cfg.interfaces[0].lspFloodStates[0]) 
    fmt.Println(lsp) 
    // SRM Flag should be set on eth0
    if ! cfg.interfaces[0].lspFloodStates[binary.BigEndian.Uint64(testLspID[:])].SRM {
        t.Fail()
    }
}

