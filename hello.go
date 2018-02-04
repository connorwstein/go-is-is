// Functions to construct/serialize hello PDUs
package main

import (
    "fmt"
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "strings"
    "unsafe"
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
    tlv_value []byte 
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
//     tlv := IsisTLV{next_tlv: nil,
//                    tlv_type: 0,
//                    tlv_length: 0,
//                    tlv_value: []byte{0x00}}

    isis_l1_lan_hello :=  IsisLANHelloPDU{header: isis_pdu_header, 
                                          circuit_type: 0x01, // 01 L1, 10 L2, 11 L1/L2
                                          source_system_id: src_system_id,
                                          holding_time: [2]byte{0x3c, 0x00}, // period a neighbor router should wait for the next IIH before declaring the original router dead, set to 60 for now
                                          pdu_length: [2]byte{0x00, 0x00}, // Whole pdu length
                                          priority: [2]byte{0x00, 0x40}, // Default priority is 64, used in the DIS election
                                          lan_dis: [7]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // Should be SID of the DIS + pseudonode id
                                          first_tlv: nil}                                        

    return &isis_l1_lan_hello // Golangs pointer analysis will allocate this on the heap
}


func serialize_isis_hello_pdu(pdu *IsisLANHelloPDU) []byte {
    // Used as the payload of an ethernet frame
    var buf bytes.Buffer
    // TLVs need to be handled specially because they can have null pointers
    // So they can't serialized the rest of the pdu in one shot, however the
    // common header can by serialized as is
    binary.Write(&buf, binary.BigEndian, pdu.header)
    binary.Write(&buf, binary.BigEndian, pdu.circuit_type)
    binary.Write(&buf, binary.BigEndian, pdu.source_system_id)
    binary.Write(&buf, binary.BigEndian, pdu.holding_time)
    binary.Write(&buf, binary.BigEndian, pdu.pdu_length)
    binary.Write(&buf, binary.BigEndian, pdu.priority)
    binary.Write(&buf, binary.BigEndian, pdu.lan_dis)
    if pdu.first_tlv != nil {
        // TODO: Will need to keep walking these tlvs until we hit the end somehow
        fmt.Println("Serializing neighbor tlv", pdu.first_tlv)
        binary.Write(&buf, binary.BigEndian, pdu.first_tlv.tlv_type)
        binary.Write(&buf, binary.BigEndian, pdu.first_tlv.tlv_length)
        binary.Write(&buf, binary.BigEndian, pdu.first_tlv.tlv_value)
    }
    return buf.Bytes()
}

func deserialize_isis_hello_pdu(raw_bytes []byte) *IsisLANHelloPDU {
    // Given the bytes received represent a hello packet,
    // construct an IsisLANHelloPDU struct with the data
    var header IsisPDUHeader
    fmt.Println(unsafe.Sizeof(header)) // Works because each member is just a byte
    // TODO: fix all these hard coded values
    // First 14 bytes are the ethernet header
    // next 8 is the common isis header
    // then circuit type then system id
    var sender_system_id [6]byte
    copy(sender_system_id[:], raw_bytes[14 + 8 + 1: 14 + 8 + 1 + 6])
    fmt.Printf("Source SID: %X%X.%X%X.%X%X\n", sender_system_id[0], sender_system_id[1], 
                sender_system_id[2], sender_system_id[3], sender_system_id[4], sender_system_id[5])
    var hello IsisLANHelloPDU
    hello.source_system_id = sender_system_id
    // 14 byte ethernet header 
    // 8 byte common isis header
    // 1+6+2+2+2+7 = 20 bytes for lan hello pdu
    var neighbor_tlv IsisTLV // Golang automagically tosses this on the heap, I like it
    tlv_offset := 14 + 8 + 20
    if raw_bytes[tlv_offset] != 0 {
        // Then we have a tlv
        fmt.Printf("TLV code %d received!\n", raw_bytes[tlv_offset]) 
        neighbor_tlv.tlv_type = raw_bytes[tlv_offset]
        neighbor_tlv.tlv_length = raw_bytes[tlv_offset + 1]
        neighbor_tlv.tlv_value = make([]byte, neighbor_tlv.tlv_length)
        copy(neighbor_tlv.tlv_value, raw_bytes[tlv_offset + 2: tlv_offset + 2 + 6])
        hello.first_tlv = &neighbor_tlv
    } else {
        hello.first_tlv = nil
    }
    return &hello
}

func send_hello(sid string, neighbors_tlv *IsisTLV) {
    // May need to send a hello with a neighbor tlv
    // Convert the sid string to an array of 6 bytes
    sid = strings.Replace(sid, ".", "", 6)
    var mybytes []byte = make([]byte, 6, 6)
    mybytes, _ = hex.DecodeString(sid)
    var fixed [6]byte
    copy(fixed[:], mybytes)

    hello_l1_lan := build_l1_iih_pdu(fixed)
    if neighbors_tlv != nil {
        fmt.Println("Send hello with neigh", neighbors_tlv)
        hello_l1_lan.first_tlv = neighbors_tlv
    }
    fmt.Println("Sending hello with tlv:", hello_l1_lan.first_tlv)
    fmt.Println(l1_lan_hello_dst)
    send_frame(build_eth_frame(l1_lan_hello_dst, get_mac("eth0"), serialize_isis_hello_pdu(hello_l1_lan)), "eth0")
}

type HelloResponse struct {
    lan_hello_pdu *IsisLANHelloPDU
    source_mac []byte
}

func recv_hello() *HelloResponse {
    // Blocks until a frame is available
    // Drop the frame unless it is the special multicast mac
    l1_lan_hello_dst = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
    hello := recv_frame("eth0") // Returns [READBUF_SIZE]byte including the full ethernet frame
    if bytes.Equal(hello[0:6], l1_lan_hello_dst) {
        fmt.Printf("Got hello from %X:%X:%X:%X:%X:%X\n", 
                   hello[6], hello[7], hello[8], hello[9], hello[10], hello[11])
        fmt.Println(hex.Dump(hello[:]))
        // Need to extract the system id from the packet  
        received_hello := deserialize_isis_hello_pdu(hello[0:len(hello)])
        var rsp HelloResponse
        rsp.lan_hello_pdu = received_hello
        rsp.source_mac = hello[6:12]
        return &rsp 
    } 
    return nil
}

