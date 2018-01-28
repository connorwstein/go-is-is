// Functions to construct/serialize hello PDUs
package main

import (
    "fmt"
    "bytes"
    "encoding/binary"
    "encoding/hex"
)

const (
    INTRA_DOMAIN_ROUTEING_PROTOCOL_DISCRIMINATOR = 0x83
    PROTOCOL_ID = 0x01
    SYSTEM_ID_LENGTH = 0x06
    L1_LAN_IIH_PDU_TYPE = 0x0F
    VERSION = 0x01
    MAX_AREA_ADDRESSES_DEFAULT = 0x00 // 0 means 3 addresses are supported
)

type IsisPDUHeader struct {
    // Common 8 byte header to all PDUs
    intra_domain_routeing_protocol_discriminator byte // 0x83
    pdu_length byte
    protocol_id byte
    system_id_length byte
    pdu_type byte // first three bits are reserved and set to 0, next 5 bits are pdu type
    version byte
    reserved byte
    maximum_area_addresses byte
}

type IsisLANHelloPDU struct {
    header IsisPDUHeader
    circuit_type byte
    source_system_id [6]byte
    holding_time [2]byte
    pdu_length [2]byte
    priority [2]byte
    lan_dis [7]byte // System ID length + 1
    first_tlv *IsisTLV // Linked list of TLVs
}

type IsisTLV struct {
    next_tlv *IsisTLV
    tlv_type byte
    tlv_length byte
    tlv_value []byte  //Size of size tlv_length
}

func build_l1_iih_pdu(src_system_id [6]byte) *IsisLANHelloPDU {
    // Takes a destination mac and builds a IsisLANHelloPDU
    // Also need a system ID for the node. Ignore the lan_dis field for now
    isis_pdu_header := IsisPDUHeader{intra_domain_routeing_protocol_discriminator: 0x83,
                                     pdu_length: 0x00, 
                                     protocol_id: 0x01, 
                                     system_id_length: 0x00, // 0 means default 6 bytes
                                     pdu_type: 0x0F, //l1 lan hello pdu
                                     version: 0x01, //
                                     reserved: 0x00,
                                     maximum_area_addresses: 0x00} // 0 means default 3 addresses
    tlv := IsisTLV{next_tlv: nil,
                   tlv_type: 0,
                   tlv_length: 0,
                   tlv_value: []byte{0x00}}

    isis_l1_lan_hello :=  IsisLANHelloPDU{header: isis_pdu_header, 
                                          circuit_type: 0x01, // 01 L1, 10 L2, 11 L1/L2
                                          source_system_id: src_system_id,
                                          holding_time: [2]byte{0x3c, 0x00}, // period a neighbor router should wait for the next IIH before declaring the original router dead, set to 60 for now
                                          pdu_length: [2]byte{0x00, 0x00}, // Whole pdu length
                                          priority: [2]byte{0x00, 0x40}, // Default priority is 64, used in the DIS election
                                          lan_dis: [7]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // Should be SID of the DIS + pseudonode id
                                          first_tlv: &tlv}                                        

    return &isis_l1_lan_hello // Golangs pointer analysis will allocate this on the heap
}


func serialize_isis_hello_pdu(pdu *IsisLANHelloPDU) []byte {
    // Used as the payload of an ethernet frame
    var buf bytes.Buffer
    // TLVs need to be handled specially because they can have null pointers
    // So can't serialized the rest of the pdu in one shot, however the
    // common header can by serialized as is
    binary.Write(&buf, binary.BigEndian, pdu.header)
    binary.Write(&buf, binary.BigEndian, pdu.circuit_type)
    binary.Write(&buf, binary.BigEndian, pdu.source_system_id)
    binary.Write(&buf, binary.BigEndian, pdu.holding_time)
    binary.Write(&buf, binary.BigEndian, pdu.pdu_length)
    binary.Write(&buf, binary.BigEndian, pdu.priority)
    binary.Write(&buf, binary.BigEndian, pdu.lan_dis)
    return buf.Bytes()
}

func send_hello() {
    hello_l1_lan := build_l1_iih_pdu([6]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11})
    fmt.Println(l1_lan_hello_dst)
    send_frame(build_eth_frame(l1_lan_hello_dst, get_mac("eth0"), serialize_isis_hello_pdu(hello_l1_lan)), "eth0")
}

func recv_hello() {
    // Blocks until a frame is available
    // Drop the frame unless it is the special multicast mac
    l1_lan_hello_dst = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
    hello := recv_frame("eth0")
    if bytes.Equal(hello[0:6],l1_lan_hello_dst) {
        fmt.Printf("Got hello from %X:%X:%X:%X:%X:%X\n", 
                   hello[6], hello[7], hello[8], hello[9], hello[10], hello[11])
        fmt.Println(hex.Dump(hello[:]))
    }
}

