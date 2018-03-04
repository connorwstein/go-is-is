package main

// import (
//     "testing"
//     "fmt"
// //     "strings"
//     "encoding/binary"
// )
// 
// func TestLocalLspGen(t *testing.T) {
//     cfg.sid = "1111.1111.1112"
//     adj := Adjacency{state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}}
// //     intf := Intf{adj: &adj}
//     initInterfaces()
//     cfg.interfaces[0].adj = &adj
// //     cfg.interfaces[0] = &intf
//     LspDBInit()
//     GenerateLocalLsp()
// 
//     // Check whether it is in the LspDB 
//     testLspID := [8]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x12, 0x00, 0x00}
//     tmp := AvlSearch(LspDB.Root, binary.BigEndian.Uint64(testLspID[:]))
//     lsp := tmp.(*IsisLsp)
//     fmt.Println(cfg.interfaces[0].lspFloodStates[0]) 
//     fmt.Println(lsp) 
//     // SRM Flag should be set on eth0
//     if ! cfg.interfaces[0].lspFloodStates[binary.BigEndian.Uint64(testLspID[:])].SRM {
//         t.Fail()
//     }
// }
// 
