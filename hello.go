// Functions to construct/serialize hello PDUs
package main

import (
    "time"
    "bytes"
    "encoding/binary"
    "encoding/hex"
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
    HELLO_INTERVAL = 4000 // Milliseconds in between hello udpates, TODO: Should be configurable
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
        glog.V(2).Info("Serializing neighbor tlv", pdu.FirstTlv)
        binary.Write(&buf, binary.BigEndian, pdu.FirstTlv.tlv_type)
        binary.Write(&buf, binary.BigEndian, pdu.FirstTlv.tlv_length)
        binary.Write(&buf, binary.BigEndian, pdu.FirstTlv.tlv_value)
    }
    glog.V(2).Info("Serialized neighbor tlv\n", hex.Dump(buf.Bytes()[:]))
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
    glog.V(2).Info("Binary decode common header:", commonHeader)
    glog.V(2).Info("Binary decode hello header:", helloHeader)
    var hello IsisLanHelloPDU
    hello.LanHelloHeader.SourceSystemId = helloHeader.SourceSystemId
    var neighborTlv IsisTLV // Golang automagically tosses this on the heap, I like it
    ethernetHeaderSize := 14
    tlv_offset := ethernetHeaderSize + int(unsafe.Sizeof(commonHeader)) + int(unsafe.Sizeof(helloHeader))
    glog.Infof("tlv offset %d raw bytes %d", tlv_offset, len(raw_bytes))
    if tlv_offset < len(raw_bytes) {
        // Then we have a tlv
        neighborTlv.tlv_type = raw_bytes[tlv_offset]
        neighborTlv.tlv_length = raw_bytes[tlv_offset + 1]
        glog.V(2).Infof("TLV code %d received, length %d!\n", raw_bytes[tlv_offset], raw_bytes[tlv_offset + 1])
        neighborTlv.tlv_value = make([]byte, neighborTlv.tlv_length)
        copy(neighborTlv.tlv_value, raw_bytes[tlv_offset + 2: tlv_offset + 2 + int(neighborTlv.tlv_length)])
        hello.FirstTlv = &neighborTlv
    } else {
        hello.FirstTlv = nil
    }
    return &hello
}

func sendHello(intf *Intf, sid string, neighbors_tlv *IsisTLV, sendChan chan []byte) {
    // May need to send a hello with a neighbor tlv
    // Convert the sid string to an array of 6 bytes
    hello_l1_lan := buildL1HelloPdu(system_id_to_bytes(sid))
    if neighbors_tlv != nil {
        hello_l1_lan.FirstTlv = neighbors_tlv
    }
    glog.V(2).Info("Sending hello with tlv:", hello_l1_lan.FirstTlv)
    sendChan <- buildEthernetFrame(l1_multicast,
                                 getMac(intf.name),
                                 serializeIsisHelloPdu(hello_l1_lan))
}

func recvHello(intf *Intf, helloChan chan []byte) *HelloResponse {
    // Blocks until a frame is available
    // Returns [READBUF_SIZE]byte including the full ethernet frame
    // This buf size needs to be big enough to at least get the length of the PDU
    // TODO: additional logic required to get the full PDU based on the length
    // in the header in case it isn't all available at once
    
    // Blocks on the hello channel
    hello := <- helloChan
    // Drop the frame unless it is the special multicast mac
    if bytes.Equal(hello[0:6], l1_multicast) {
        glog.V(2).Infof("Got hello from %X:%X:%X:%X:%X:%X\n",
                   hello[6], hello[7], hello[8], hello[9], hello[10], hello[11])
        glog.V(2).Infof(hex.Dump(hello[:]))
        // Need to extract the system id from the packet
        received_hello := deserializeIsisHelloPdu(hello[0:len(hello)])
        var rsp HelloResponse
        rsp.lan_hello_pdu = received_hello
        rsp.source_mac = hello[6:12]
        return &rsp
    }
    return nil
}

func isisHelloSend(intf *Intf, sendChan chan []byte) {
    // Send hellos every HELLO_INTERVAL after a system ID has been configured
	// on the specified interface
    for {
        glog.V(2).Infof("Locking interface and config %s", intf.name)
        intf.lock.Lock()
        cfg.lock.Lock()
        if cfg.sid != "" {
            glog.Infof("Adjacency state on %v: %v goroutine ID %d", intf.name, intf.adj.state, getGID())
            if intf.adj.state != "UP" {
                sendHello(intf, cfg.sid, nil, sendChan)
            }
        }
        glog.V(2).Infof("Unlocking interface and config %s", intf.name)
        cfg.lock.Unlock()
        intf.lock.Unlock()
        time.Sleep(HELLO_INTERVAL * time.Millisecond)
    }
}

func isisHelloRecv(intf *Intf, helloChan chan []byte, sendChan chan []byte) {
    // Forever receiving hellos on the passed interface
    // Updating the status of the interface as an adjacency is
    // established
    for {
        // Blocking call to read
        rsp := recvHello(intf, helloChan)
        // Can get a nil response for ethernet frames received
        // which are not destined for the IS-IS hello multicast address
        if rsp == nil {
            continue
        }
        intf.lock.Lock()
        glog.Info("Receving on intf: ", intf.name, " goroutine ID ", getGID())
        intf.lock.Unlock()
        // Depending on what type of hello it is, respond
        // Respond to this hello packet with a IS-Neighbor TLV
        // If we receive a hello with no neighbor tlv, we copy
        // the mac of the sender into the neighbor tlv and send it back out
        // then mark the adjacency on that interface as INITIALIZING
        // If we receive a hello with our own mac in the neighbor tlv
        // we mark the adjacency as UP
        glog.Infof("Got hello from %v\n", system_id_to_str(rsp.lan_hello_pdu.LanHelloHeader.SourceSystemId[:]))
        // This should not be our own system id, drop it if it is
        if system_id_to_str(rsp.lan_hello_pdu.LanHelloHeader.SourceSystemId[:]) == cfg.sid {
            glog.Infof("Got hello from our own system ID, dropping\n")
            continue
        }
        // even if our adjacency is up, we need to respond to other folks
        if rsp.lan_hello_pdu.FirstTlv == nil {
            // No TLVs yet in this hello packet so we need to add in the IS neighbors tlv
            // TLV type 6
            // After getting this --> adjacency is in the initializing state
            var neighbors_tlv IsisTLV
            neighbors_tlv.next_tlv = nil
            neighbors_tlv.tlv_type = 6
            neighbors_tlv.tlv_length = 6 // Just one other mac for now
            neighbors_tlv.tlv_value = rsp.source_mac // []byte of the senders mac address
            intf.lock.Lock()
            if intf.adj.state != "UP" {
                glog.Infof("Initializing adjacency on intf %v", intf.name)
                intf.adj.state = "INIT"
            }
            intf.lock.Unlock()
            // Send a hello back out the interface we got the response on
            // But with the neighbor tlv
            sendHello(intf, cfg.sid, &neighbors_tlv, sendChan)
        } else {
            // If we do have the neighbors tlv, check if it has our own mac in it
            // if it does then we know the adjacency is established
            intf.lock.Lock()
            if bytes.Equal(rsp.lan_hello_pdu.FirstTlv.tlv_value, getMac(intf.name)) {
                intf.adj.state = "UP"
                intf.adj.metric = 10
                intf.adj.neighbor_system_id = make([]byte, 6)
                copy(intf.adj.neighbor_system_id, rsp.lan_hello_pdu.LanHelloHeader.SourceSystemId[:])
                glog.Infof("Adjacency up between %v and %v on intf %v", cfg.sid, system_id_to_str(intf.adj.neighbor_system_id), intf.name)
                // Signal that an adjacency change has occurred, so we should regenerate our lsp
                // and flood
                // Optimization might be to use this adjacency information to only update that part of the 
                // LSP, rather than rebuilding the whole thing from the adjacency database
                intf.lock.Unlock()
                GenerateLocalLsp()
            } else {
                intf.lock.Unlock()
            }
        }
    }
}
