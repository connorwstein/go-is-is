// Functions to construct/serialize hello PDUs
package main

import (
//     "fmt"
    "bytes"
    "encoding/binary"
    "encoding/hex"
    "strings"
    "unsafe"
    "github.com/golang/glog"
)

const (
    INTRA_DOMAIN_ROUTEING_PROTOCOL_DISCRIMINATOR = 0x83
    PROTOCOL_ID = 0x01
    SYSTEM_ID_LENGTH = 0x06
    L1_LAN_IIH_PDU_TYPE = 0x0F
    VERSION = 0x01
    MAX_AREA_ADDRESSES_DEFAULT = 0x00 // 0 means 3 addresses are supported
)

type IsisLanHelloHeader struct {
    CircuitType byte
    SourceSystemId [6]byte
    HoldingTime [2]byte
    PduLength [2]byte
    Priority [2]byte
    LanDis [7]byte // System ID length + 1
}

type IsisLanHelloPDU struct {
    Header IsisPDUHeader
    LanHelloHeader IsisLanHelloHeader
    FirstTlv *IsisTLV // Linked list of TLVs
}

type IsisTLV struct {
    next_tlv *IsisTLV
    tlv_type byte
    tlv_length byte
    tlv_value []byte
}

type HelloResponse struct {
    intf *Intf
    lan_hello_pdu *IsisLanHelloPDU
    source_mac []byte
}

func buildL1HelloPdu(src_system_id [6]byte) *IsisLanHelloPDU {
    // Takes a destination mac and builds a IsisLanHelloPDU
    // Also need a system ID for the node. Ignore the lan_dis field for now
    isis_pdu_header := IsisPDUHeader{Intra_domain_routeing_protocol_discriminator: 0x83,
                                     Pdu_length: 0x00,
                                     Protocol_id: 0x01,
                                     System_id_length: 0x00, // 0 means default 6 bytes
                                     Pdu_type: 0x0F, //l1 lan hello pdu
                                     Version: 0x01, //
                                     Reserved: 0x00,
                                     Maximum_area_addresses: 0x00} // 0 means default 3 addresses

    isis_lan_hello_header :=  IsisLanHelloHeader{
                                          CircuitType: 0x01, // 01 L1, 10 L2, 11 L1/L2
                                          SourceSystemId: src_system_id,
                                          HoldingTime: [2]byte{0x3c, 0x00}, // period a neighbor router should wait for the next IIH before declaring the original router dead, set to 60 for now
                                          PduLength: [2]byte{0x00, 0x00}, // Whole pdu length
                                          Priority: [2]byte{0x00, 0x40}, // Default priority is 64, used in the DIS election
                                          LanDis: [7]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // Should be SID of the DIS + pseudonode id
                                          }

    var isis_l1_lan_hello IsisLanHelloPDU
    isis_l1_lan_hello.Header = isis_pdu_header
    isis_l1_lan_hello.LanHelloHeader = isis_lan_hello_header
    isis_l1_lan_hello.FirstTlv = nil
    return &isis_l1_lan_hello // Golangs pointer analysis will allocate this on the heap
}


func serializeIsisHelloPdu(pdu *IsisLanHelloPDU) []byte {
    // Used as the payload of an ethernet frame
    var buf bytes.Buffer
    // TLVs need to be handled specially because they can have null pointers
    // So they can't serialized the rest of the pdu in one shot, however the
    // common header can by serialized as is
    binary.Write(&buf, binary.BigEndian, pdu.Header)
    binary.Write(&buf, binary.BigEndian, pdu.LanHelloHeader)
    if pdu.FirstTlv != nil {
        // TODO: Will need to keep walking these tlvs until we hit the end somehow
        glog.Info("Serializing neighbor tlv", pdu.FirstTlv)
        binary.Write(&buf, binary.BigEndian, pdu.FirstTlv.tlv_type)
        binary.Write(&buf, binary.BigEndian, pdu.FirstTlv.tlv_length)
        binary.Write(&buf, binary.BigEndian, pdu.FirstTlv.tlv_value)
    }
    return buf.Bytes()
}

func deserializeIsisHelloPdu(raw_bytes []byte) *IsisLanHelloPDU {
    // Given the bytes received represent a hello packet,
    // construct an IsisLanHelloPDU struct with the data
    // We can skip the whole ethernet header
    // So raw_bytes[14:] is all we are interested in
    buf := bytes.NewBuffer(raw_bytes[14:])
    var commonHeader IsisPDUHeader 
    var helloHeader IsisLanHelloHeader
    binary.Read(buf, binary.BigEndian, &commonHeader)
    binary.Read(buf, binary.BigEndian, &helloHeader)
    glog.Info("Binary decode common header:", commonHeader)
    glog.Info("Binary decode hello header:", helloHeader)
    var hello IsisLanHelloPDU
    hello.LanHelloHeader.SourceSystemId = helloHeader.SourceSystemId
    var neighborTlv IsisTLV // Golang automagically tosses this on the heap, I like it
    ethernetHeaderSize := 14
    tlv_offset := ethernetHeaderSize + int(unsafe.Sizeof(commonHeader)) + int(unsafe.Sizeof(helloHeader))
    if raw_bytes[tlv_offset] != 0 {
        // Then we have a tlv
        glog.Infof("TLV code %d received!\n", raw_bytes[tlv_offset])
        neighborTlv.tlv_type = raw_bytes[tlv_offset]
        neighborTlv.tlv_length = raw_bytes[tlv_offset + 1]
        neighborTlv.tlv_value = make([]byte, neighborTlv.tlv_length)
        copy(neighborTlv.tlv_value, raw_bytes[tlv_offset + 2: tlv_offset + 2 + 6])
        hello.FirstTlv = &neighborTlv
    } else {
        hello.FirstTlv = nil
    }
    return &hello
}

func sendHello(intf *Intf, sid string, neighbors_tlv *IsisTLV) {
    // May need to send a hello with a neighbor tlv
    // Convert the sid string to an array of 6 bytes
    sid = strings.Replace(sid, ".", "", 6)
    var mybytes []byte = make([]byte, 6, 6)
    mybytes, _ = hex.DecodeString(sid)
    var fixed [6]byte
    copy(fixed[:], mybytes)

    hello_l1_lan := buildL1HelloPdu(fixed)
    if neighbors_tlv != nil {
        hello_l1_lan.FirstTlv = neighbors_tlv
    }
    glog.Info("Sending hello with tlv:", hello_l1_lan.FirstTlv)
    sendFrame(buildEthernetFrame(l1_lan_hello_dst,
                                 getMac(intf.name),
                                 serializeIsisHelloPdu(hello_l1_lan)), intf.name)
}

func recvHello(intf *Intf, helloChan chan [READ_BUF_SIZE]byte) *HelloResponse {
    // Blocks until a frame is available
    // Returns [READBUF_SIZE]byte including the full ethernet frame
    // This buf size needs to be big enough to at least get the length of the PDU
    // TODO: additional logic required to get the full PDU based on the length
    // in the header in case it isn't all available at once
    
    // Blocks on the hello channel
    hello := <- helloChan

    // Drop the frame unless it is the special multicast mac
    if bytes.Equal(hello[0:6], l1_lan_hello_dst) {
        glog.Infof("Got hello from %X:%X:%X:%X:%X:%X\n",
                   hello[6], hello[7], hello[8], hello[9], hello[10], hello[11])
        glog.Infof(hex.Dump(hello[:]))
        // Need to extract the system id from the packet
        received_hello := deserializeIsisHelloPdu(hello[0:len(hello)])
        var rsp HelloResponse
        rsp.lan_hello_pdu = received_hello
        rsp.source_mac = hello[6:12]
        rsp.intf = intf
        return &rsp
    }
    return nil
}
