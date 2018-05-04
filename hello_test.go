package main
import (
    "testing"
    "net"
    "bytes"
)

func TestInterfaceTLV(t *testing.T){
    i := Intf{prefix: net.IP{0x01, 0x01, 0x01, 0x02}}    
    t.Log(i)
    tlv := getInterfaceTLV(&i)
    t.Log(tlv)
    if ! bytes.Equal(tlv.valueTLV, []byte{0x01,0x01, 0x01, 0x02}) {
        t.Fail()
    }
}

