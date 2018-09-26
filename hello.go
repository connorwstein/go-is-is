// Hello process in the IS-IS protocol.
// Establishes adjacencies with neighbors.
// +build linux

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/golang/glog"
	"net"
	"time"
	"unsafe"
)

const (
	INTRA_DOMAIN_ROUTEING_PROTOCOL_DISCRIMINATOR = 0x83
	PROTOCOL_ID                                  = 0x01
	SYSTEM_ID_LENGTH                             = 0x06
	L1_LAN_IIH_PDU_TYPE                          = 0x0F
	VERSION                                      = 0x01
	MAX_AREA_ADDRESSES_DEFAULT                   = 0x00 // 0 means 3 addresses are supported
	HELLO_INTERVAL                               = 4000 // Milliseconds in between hello udpates, TODO: Should be configurable
)

type IsisLanHelloHeader struct {
	// Fields need to be exported for the binary encoding
	CircuitType    byte
	SourceSystemID [6]byte
	HoldingTime    [2]byte
	LengthPDU      [2]byte
	Priority       [2]byte
	LanDis         [7]byte // System ID length + 1
}

type IsisLanHelloPDU struct {
	Header         IsisPDUHeader
	LanHelloHeader IsisLanHelloHeader
	FirstTLV       *IsisTLV // Linked list of TLVs
}

type HelloResponse struct {
	intf        *Intf
	lanHelloPDU *IsisLanHelloPDU
	sourceMac   []byte
}

func buildL1HelloPDU(srcSystemID [6]byte) *IsisLanHelloPDU {
	// Takes a destination mac and builds a IsisLanHelloPDU
	// Also need a system ID for the node. Ignore the lan_dis field for now
	isis_pdu_header := IsisPDUHeader{IntraDomainRouteingProtocolDiscriminator: 0x83,
		LengthPDU:            0x00,
		ProtocolID:           0x01,
		SystemIDLength:       0x00, // 0 means default 6 bytes
		TypePDU:              0x0F, //l1 lan hello pdu
		Version:              0x01, //
		Reserved:             0x00,
		MaximumAreaAddresses: 0x00} // 0 means default 3 addresses

	isis_lan_hello_header := IsisLanHelloHeader{
		CircuitType:    0x01, // 01 L1, 10 L2, 11 L1/L2
		SourceSystemID: srcSystemID,
		HoldingTime:    [2]byte{0x3c, 0x00},                               // period a neighbor router should wait for the next IIH before declaring the original router dead, set to 60 for now
		LengthPDU:      [2]byte{0x00, 0x00},                               // Whole pdu length
		Priority:       [2]byte{0x00, 0x40},                               // Default priority is 64, used in the DIS election
		LanDis:         [7]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // Should be SID of the DIS + pseudonode id
	}

	var isis_l1_lan_hello IsisLanHelloPDU
	isis_l1_lan_hello.Header = isis_pdu_header
	isis_l1_lan_hello.LanHelloHeader = isis_lan_hello_header
	isis_l1_lan_hello.FirstTLV = nil
	return &isis_l1_lan_hello // Golangs pointer analysis will allocate this on the heap
}

func serializeIsisHelloPDU(pdu *IsisLanHelloPDU) []byte {
	// Used as the payload of an ethernet frame
	var buf bytes.Buffer
	// TLVs need to be handled specially because they can have null pointers
	// So they can't serialized the rest of the pdu in one shot, however the
	// common header can by serialized as is
	binary.Write(&buf, binary.BigEndian, pdu.Header)
	binary.Write(&buf, binary.BigEndian, pdu.LanHelloHeader)
	tlv := pdu.FirstTLV
	for tlv != nil {
		// TODO: Will need to keep walking these tlvs until we hit the end somehow
		glog.V(2).Info("Serializing tlv:", tlv.typeTLV)
		binary.Write(&buf, binary.BigEndian, tlv.typeTLV)
		binary.Write(&buf, binary.BigEndian, tlv.lengthTLV)
		binary.Write(&buf, binary.BigEndian, tlv.valueTLV)
		tlv = tlv.nextTLV
	}
	return buf.Bytes()
}

func deserializeIsisHelloPDU(raw_bytes []byte) *IsisLanHelloPDU {
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
	hello.LanHelloHeader.SourceSystemID = helloHeader.SourceSystemID
	ethernetHeaderSize := 14
	tlv_offset := ethernetHeaderSize + int(unsafe.Sizeof(commonHeader)) + int(unsafe.Sizeof(helloHeader))
	glog.Infof("tlv offset %d raw bytes %d", tlv_offset, len(raw_bytes))
	hello.FirstTLV = parseTLVs(raw_bytes, tlv_offset)
	return &hello
}

func getInterfaceTLV(intf *Intf) *IsisTLV {
	var interfaceTLV IsisTLV
	interfaceTLV.typeTLV = ISIS_IP_INTF_ADDR_TLV
	interfaceTLV.lengthTLV = 4
	interfaceTLV.valueTLV = make([]byte, 4)
	copy(interfaceTLV.valueTLV, intf.prefix.To4())
	glog.V(2).Infof("Interface TLV 132 %v", interfaceTLV)
	return &interfaceTLV
}

func sendHello(intf *Intf, sid string, neighborsTLV *IsisTLV, sendChan chan []byte) {
	// May need to send a hello with a neighbor tlv
	// Convert the sid string to an array of 6 bytes
	hello_l1_lan := buildL1HelloPDU(systemIDToBytes(sid))
	if neighborsTLV != nil {
		hello_l1_lan.FirstTLV = neighborsTLV
		hello_l1_lan.FirstTLV.nextTLV = getInterfaceTLV(intf)
		// Need to also add TLV 132 which has the outgoing ip address
		glog.V(2).Infof("Sending hello with tlvs %v %v", hello_l1_lan.FirstTLV, hello_l1_lan.FirstTLV.nextTLV)
	}
	sendChan <- buildEthernetFrame(l1_multicast,
		getMac(intf.name),
		serializeIsisHelloPDU(hello_l1_lan))
}

func recvHello(intf *Intf, helloChan chan []byte) *HelloResponse {
	// Blocks until a frame is available
	// Returns [READBUF_SIZE]byte including the full ethernet frame
	// This buf size needs to be big enough to at least get the length of the PDU
	// TODO: additional logic required to get the full PDU based on the length
	// in the header in case it isn't all available at once

	// Blocks on the hello channel
	hello := <-helloChan
	// Drop the frame unless it is the special multicast mac
	if bytes.Equal(hello[0:6], l1_multicast) {
		glog.V(2).Infof("Got hello from %X:%X:%X:%X:%X:%X\n",
			hello[6], hello[7], hello[8], hello[9], hello[10], hello[11])
		glog.V(4).Infof(hex.Dump(hello[:]))
		// Need to extract the system id from the packet
		received_hello := deserializeIsisHelloPDU(hello[0:len(hello)])
		var rsp HelloResponse
		rsp.lanHelloPDU = received_hello
		rsp.sourceMac = hello[6:12]
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
		glog.Infof("Got hello from %v\n", systemIDToString(rsp.lanHelloPDU.LanHelloHeader.SourceSystemID[:]))
		// This should not be our own system id, drop it if it is
		if systemIDToString(rsp.lanHelloPDU.LanHelloHeader.SourceSystemID[:]) == cfg.sid {
			glog.Infof("Got hello from our own system ID, dropping\n")
			continue
		}
		// even if our adjacency is up, we need to respond to other folks
		if rsp.lanHelloPDU.FirstTLV == nil {
			// No TLVs yet in this hello packet so we need to add in the IS neighbors tlv
			// TLV type 6
			// After getting this --> adjacency is in the initializing state
			var neighborsTLV IsisTLV
			neighborsTLV.nextTLV = nil
			neighborsTLV.typeTLV = 6
			neighborsTLV.lengthTLV = 6            // Just one other mac for now
			neighborsTLV.valueTLV = rsp.sourceMac // []byte of the senders mac address
			intf.lock.Lock()
			if intf.adj.state != "UP" {
				glog.Infof("Initializing adjacency on intf %v", intf.name)
				intf.adj.state = "INIT"
			}
			intf.lock.Unlock()
			// Send a hello back out the interface we got the response on
			// But with the neighbor tlv
			sendHello(intf, cfg.sid, &neighborsTLV, sendChan)
		} else {
			// If we do have the neighbors tlv, check if it has our own mac in it
			// if it does then we know the adjacency is established
			intf.lock.Lock()
			if bytes.Equal(rsp.lanHelloPDU.FirstTLV.valueTLV, getMac(intf.name)) {
				intf.adj.state = "UP"
				intf.adj.metric = 10
				intf.adj.neighborSystemID = make([]byte, 6)
				intf.adj.neighborIP = make(net.IP, 4)
				copy(intf.adj.neighborSystemID, rsp.lanHelloPDU.LanHelloHeader.SourceSystemID[:])
				copy(intf.adj.neighborIP, net.IP(rsp.lanHelloPDU.FirstTLV.nextTLV.valueTLV))
				glog.Infof("Adjacency up between %v and %v on intf %v, neighbor IP %v", cfg.sid, systemIDToString(intf.adj.neighborSystemID), intf.name, intf.adj.neighborIP)
				// Signal that an adjacency change has occurred, so we should regenerate our lsp
				// and flood
				// Optimization might be to use this adjacency information to only update that part of the
				// LSP, rather than rebuilding the whole thing from the adjacency database
				intf.lock.Unlock()
				generateLocalLsp()
			} else {
				intf.lock.Unlock()
			}
		}
	}
}
