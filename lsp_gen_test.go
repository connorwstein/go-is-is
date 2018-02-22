package main

import (
    "testing"
//     "fmt"
    "strings"
)

func TestLocalLSPGen(t *testing.T) {
    cfg.sid = "1111.1111.1112"
    adj := Adjacency{state: "UP", neighbor_system_id: []byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11}}
    intf := Intf{adj: &adj}
    GenerateLocalLsp(&intf, true)
    if strings.Compare(system_id_to_str(lsps[0].LspID[:6]), cfg.sid) != 0 {
        t.Fail()
    }
    GenerateLocalLsp(&intf, true)
    if strings.Compare(system_id_to_str(lsps[1].LspID[:6]), cfg.sid) != 0 {
        t.Fail()
    }
}

