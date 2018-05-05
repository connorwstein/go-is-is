// Update process in the IS-IS protocol.
// Generates local LSPs, receives remote LSPs and floods them appropriately. Builds
// the update database.
// +build linux

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/golang/glog"
	"net"
	"sync"
	"time"
	"unsafe"
)

const (
	LSP_REFRESH = 5000
)

var UpdateDB *IsisDB
var sequenceNumber uint32

type IsisLspHeader struct {
	LengthPDU         [2]byte
	RemainingLifetime [2]byte
	LspID             [8]byte // System id (6 bytes) + 1 byte PSN + 1 byte fragment
	SequenceNumber    [4]byte
	Checksum          [2]byte
	PAttOLType        byte
}

type IsisLspCore struct {
	Header    IsisPDUHeader
	LspHeader IsisLspHeader
	FirstTLV  *IsisTLV
}

type IsisLsp struct {
	// Wrapper struct around the core
	Key     uint64 // Used for key in the UpdateDB
	LspID   [8]byte
	CoreLsp *IsisLspCore
}

func (lsp IsisLsp) String() string {
	var lspString bytes.Buffer
	lspString.WriteString(fmt.Sprintf("%s", systemIDToString(lsp.LspID[:6])))
	var curr *IsisTLV = lsp.CoreLsp.FirstTLV
	for curr != nil {
		lspString.WriteString(fmt.Sprintf("\tTLV %d\n", curr.typeTLV))
		lspString.WriteString(fmt.Sprintf("\tTLV size %d\n", curr.lengthTLV))
		if curr.typeTLV == ISIS_IP_INTERNAL_REACH_TLV {
			// This is a external reachability tlv
			// TODO: fix hard coding here
			for i := 0; i < int(curr.lengthTLV)/12; i++ {
				var prefix net.IPNet
				prefix.IP = curr.valueTLV[i*12 : i*12+4]
				prefix.Mask = curr.valueTLV[i*12+4 : i*12+8]
				metric := curr.valueTLV[i*12+8 : i*12+12]
				lspString.WriteString(fmt.Sprintf("\t\t%s Metric %d\n", prefix.String(), binary.BigEndian.Uint32(metric[:])))
			}
		} else if curr.typeTLV == ISIS_NEIGHBORS_TLV {
			// This is a neighbors tlv, its length - 1 (to exclude the first virtualByteFlag) will be a multiple of 11
			for i := 0; i < int(curr.lengthTLV-1)/11; i++ {
				// print out the neighbor system ids and metric
				metric := curr.valueTLV[i*11+4]
				systemID := curr.valueTLV[(i*11 + 1 + 4):(i*11 + 1 + 4 + 6)]
				lspString.WriteString(fmt.Sprintf("\t\t%s Metric %d\n", systemIDToString(systemID), metric))
			}
		}
		curr = curr.nextTLV
	}
	return lspString.String()
}

type IsisDB struct {
	DBLock sync.Mutex
	Root   *AvlNode
	// May want to add more information here
}

type Neighbor struct {
	systemID string
	metric   uint32
}

func updateDBInit() {
	UpdateDB = &IsisDB{DBLock: sync.Mutex{}, Root: nil}
}

func floodNewLsp(receiveIntf *Intf, receivedLsp *IsisLsp) {
	// Add this new LSP to all interfaces floodStates, and set SRM to true for all of them EXCEPT this interface which we
	// received it from
	for _, intf := range cfg.interfaces {
		glog.V(2).Infof("Locking interface %s", intf.name)
		intf.lock.Lock()
		if receiveIntf.name == intf.name {
			// If it is already there, just set SRM to true
			if _, inMap := intf.lspFloodStates[receivedLsp.Key]; !inMap {
				intf.lspFloodStates[receivedLsp.Key] = &LspFloodState{LspIDKey: receivedLsp.Key, LspID: receivedLsp.LspID, SRM: false, SSN: false}
			} else {
				intf.lspFloodStates[receivedLsp.Key].SRM = false
			}
		} else {
			glog.Infof("Flooding new lsp %s out interface: %s", systemIDToString(receivedLsp.LspID[:6]), intf.name)
			// If it is already there, just set SRM to true
			if _, inMap := intf.lspFloodStates[receivedLsp.Key]; !inMap {
				intf.lspFloodStates[receivedLsp.Key] = &LspFloodState{LspIDKey: receivedLsp.Key, LspID: receivedLsp.LspID, SRM: true, SSN: false}
			} else {
				intf.lspFloodStates[receivedLsp.Key].SRM = true
			}
		}
		glog.V(2).Infof("Unlocking interface %s", intf.name)
		intf.lock.Unlock()
	}
}

func isisUpdateInput(receiveIntf *Intf, update chan []byte, triggerSPF chan bool) {
	// Need to flood it along to every interface, except the one it came from
	// The one it came from is the one we are listening on
	// This lsp is a raw buffer [READ_BUF_SIZE]byte, need to deserialize
	for {
		lsp := <-update
		receivedLsp := deserializeLsp(lsp[:])
		glog.V(2).Infof("Got lsp update %s sequence number %d", systemIDToString(receivedLsp.LspID[:6]), binary.BigEndian.Uint32(receivedLsp.CoreLsp.LspHeader.SequenceNumber[:]))
		glog.V(4).Infof(hex.Dump(lsp[:]))
		// Check if we already have this LSP, if not, then insert it
		// into our own DB an flood it along to all the other interfaces we have
		// If we already have a copy and the sequence number is newer, overwrite.
		// TODO: If we have a newer copy, send the newer copy back to the source
		spf := false
		UpdateDB.DBLock.Lock()
		tmp := AvlSearch(UpdateDB.Root, receivedLsp.Key)
		if tmp == nil {
			// Don't have this LSP so lets add it
			glog.Infof("Adding new lsp %s (%v) to DB", systemIDToString(receivedLsp.LspID[:6]), receivedLsp.Key)
			UpdateDB.Root = AvlInsert(UpdateDB.Root, receivedLsp.Key, receivedLsp, false)
			printUpdateDB(UpdateDB.Root)
			// Receiving a brand new LSP triggers an SPF
			spf = true
			floodNewLsp(receiveIntf, receivedLsp)
		} else {
			// We do have this LSP, check if the sequence number is newer than the current version we have if it is then update
			lsp := tmp.(*IsisLsp)
			if binary.BigEndian.Uint32(lsp.CoreLsp.LspHeader.SequenceNumber[:]) < binary.BigEndian.Uint32(receivedLsp.CoreLsp.LspHeader.SequenceNumber[:]) {
				// Received one is newer, update and flood
				glog.Infof("Overwriting new lsp %s (%v) to DB", systemIDToString(receivedLsp.LspID[:6]), receivedLsp.Key)
				UpdateDB.Root = AvlInsert(UpdateDB.Root, receivedLsp.Key, receivedLsp, true)
				printUpdateDB(UpdateDB.Root)
				// Receiving newer LSP also triggers an SPF
				spf = true
				floodNewLsp(receiveIntf, receivedLsp)
			}
		}
		UpdateDB.DBLock.Unlock()
		glog.V(2).Infof("SPF trigger %v", spf)
		triggerSPF <- spf
	}
}

func isisUpdate(intf *Intf, send chan []byte) {
	for {
		glog.V(2).Infof("Locking interface %s", intf.name)
		intf.lock.Lock()
		glog.Info("LSP DB:")
		printUpdateDB(UpdateDB.Root)
		glog.Infof("Intf %s Flood States", intf.name)
		printLspFloodStates(intf)
		// Check for SRM == true on this interface, if there
		// then use the key to get the full LSP, send it and clear the flag
		for _, lspFloodState := range intf.lspFloodStates {
			// Need the adjacency to be UP as well
			if lspFloodState.SRM && intf.adj.state == "UP" {
				tmp := AvlSearch(UpdateDB.Root, lspFloodState.LspIDKey)
				if tmp == nil {
					glog.Errorf("Unable to find %s (%v) in lsp db", systemIDToString(lspFloodState.LspID[:6]), lspFloodState.LspIDKey)
					glog.Errorf("Lsp DB:")
					printUpdateDB(UpdateDB.Root)
				} else {
					lsp := tmp.(*IsisLsp)
					// Send it out that particular interface
					glog.Infof("Flooding %s out %s", systemIDToString(lspFloodState.LspID[:6]), intf.name)
					send <- buildEthernetFrame(l1_multicast, getMac(intf.name), serializeLsp(lsp.CoreLsp))
					// No ACK required for LAN interfaces
					lspFloodState.SRM = false
				}
			}
		}
		glog.V(2).Infof("Unlocking interface %s", intf.name)
		intf.lock.Unlock()
		time.Sleep(LSP_REFRESH * time.Millisecond)
	}
}

func deserializeLsp(raw_bytes []byte) *IsisLsp {
	// Given the raw buffer received, build an IsisLsp structure
	buf := bytes.NewBuffer(raw_bytes[14:])
	var commonHeader IsisPDUHeader
	var lspHeader IsisLspHeader
	binary.Read(buf, binary.BigEndian, &commonHeader)
	binary.Read(buf, binary.BigEndian, &lspHeader)
	glog.V(2).Info("Binary decode common header:", commonHeader)
	glog.V(2).Info("Binary decode lsp header:", lspHeader)
	var coreLsp *IsisLspCore = &IsisLspCore{Header: commonHeader, LspHeader: lspHeader, FirstTLV: nil}
	ethernetHeaderSize := 14
	tlv_offset := ethernetHeaderSize + int(unsafe.Sizeof(commonHeader)) + int(unsafe.Sizeof(lspHeader))
	// Check if the tlv offset is strictly less than the raw bytes, if it is then there must be TLVs present
	// keep reading until remaining tlv data is 0, building up a linked list of the TLVs as we go
	remainingTLVBytes := len(raw_bytes) - tlv_offset
	glog.V(2).Infof("Received %d raw bytes not including ethernet header. TLV bytes %d. TLV offset %d", len(raw_bytes), remainingTLVBytes, tlv_offset)
    coreLsp.FirstTLV = parseTLVs(raw_bytes, tlv_offset)
	var lsp IsisLsp = IsisLsp{Key: lspIDToKey(lspHeader.LspID), LspID: lspHeader.LspID, CoreLsp: coreLsp}
	return &lsp
}

func serializeLsp(lsp *IsisLspCore) []byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, lsp.Header)
	binary.Write(&buf, binary.BigEndian, lsp.LspHeader)
	tlv := lsp.FirstTLV
	for tlv != nil {
		glog.V(2).Info("Serializing tlv:", tlv.typeTLV)
		binary.Write(&buf, binary.BigEndian, tlv.typeTLV)
		binary.Write(&buf, binary.BigEndian, tlv.lengthTLV)
		binary.Write(&buf, binary.BigEndian, tlv.valueTLV)
		tlv = tlv.nextTLV
	}
	return buf.Bytes()
}

func systemIDToKey(systemID string) uint64 {
	var lspID [8]byte
	bytes := systemIDToBytes(systemID)
	copy(lspID[:], bytes[:])
	return lspIDToKey(lspID)
}

func lspIDToKey(lspID [8]byte) uint64 {
	var key uint64 = binary.BigEndian.Uint64(lspID[:]) // Keyed on the LSP ID's integer value
	return key
}

func systemIDToLspID(systemID string) [8]byte {
	var lspID [8]byte
	bytes := systemIDToBytes(systemID)
	copy(lspID[:], bytes[:])
	return lspID
}

func getNeighbors(neighborTLV *IsisTLV) []*Neighbor {
	var neighbors []*Neighbor
	neighborCount := (int(neighborTLV.lengthTLV) - 1) / 11
	glog.V(2).Infof("Neighbor count %d", neighborCount)
	neighbors = make([]*Neighbor, neighborCount)
	currentNeighbor := 0
	currentByte := 1 // Skip first virtual byte
	for currentNeighbor < neighborCount {
		// TLV value is 1 virtual byte flag and then n multiples of 4 byte metric and 6 byte system id + 1 byte pseudo-node id
		// Set pseudo-node id to 0 for now
		var neighbor Neighbor
		neighbor.metric = binary.BigEndian.Uint32(neighborTLV.valueTLV[currentByte : currentByte+4])
		currentByte += 4
		neighbor.systemID = systemIDToString(neighborTLV.valueTLV[currentByte : currentByte+6])
		currentByte += 7 // Skip PSN
		glog.V(2).Infof("Current byte %d", currentByte)
		neighbors[currentNeighbor] = &neighbor
		currentNeighbor++
	}
	return neighbors
}

func lookupNeighbors(lsp *IsisLsp) []*Neighbor {
	// Given an LSP returns list of neighbors
	currentTLV := lsp.CoreLsp.FirstTLV
	for currentTLV != nil {
		if int(currentTLV.typeTLV) == ISIS_NEIGHBORS_TLV {
			return getNeighbors(currentTLV)
		}
		currentTLV = currentTLV.nextTLV
	}
	glog.V(2).Infof("No neighbor tlv found in LSP %s", systemIDToString(lsp.LspID[:6]))
	return nil
}

func getIPReachTLV(interfaces []*Intf) *IsisTLV {
	// Doesn't handle duplicate prefixes reachable via different interfaces
	var ipReachTLV IsisTLV
	ipReachTLV.nextTLV = nil
	ipReachTLV.typeTLV = 128
	ipReachTLV.lengthTLV = 0 // Number of directly connected prefixes * 12 bytes
	// For each interface, need to look up the routes associated
	for _, intf := range interfaces {
		// Routes are already saved in each interface struct
		for _, route := range intf.routes {
			// Dst will be nil for loopback
			if route != nil {
				// Add this route to the TLV
				// 4 bytes metric information
				// 4 bytes for ip prefix
				// 4 bytes for ip subnet mask
				ipReachTLV.valueTLV = append(ipReachTLV.valueTLV, route.IP[:]...)
				ipReachTLV.valueTLV = append(ipReachTLV.valueTLV, route.Mask[:]...)
				metric := [4]byte{0x00, 0x00, 0x00, 0x0a}
				ipReachTLV.valueTLV = append(ipReachTLV.valueTLV, metric[:]...) // Using metric of 10 always (1 hop)
                glog.V(2).Infof("Adding route %v", route)
				ipReachTLV.lengthTLV += 12
			}
		}
	}
	return &ipReachTLV
}

func getNeighborTLV(interfaces []*Intf) *IsisTLV {
	var neighborsTLV IsisTLV
	neighborsTLV.nextTLV = nil
	neighborsTLV.typeTLV = 2
	neighborsTLV.lengthTLV = 1 // Start at 1 to include virtual byte flag
	var virtualByteFlag byte = 0x00
	var pseudoNodeId byte = 0x00
	neighborsTLV.valueTLV = append(neighborsTLV.valueTLV, virtualByteFlag)
	// Loop though the interfaces looking for adjacencies to append
	for _, intf := range interfaces {
		// TLV value is 1 virtual byte flag and then n multiples of 4 byte metric and 6 byte system id + 1 byte pseudo-node id
		// Set pseudo-node id to 0 for now
		if intf.adj.state != "UP" {
			// Only send the adjacencies that we actually have
			continue
		}
		metric := [4]byte{0x00, 0x00, 0x00, 0x0a}
		neighborsTLV.valueTLV = append(neighborsTLV.valueTLV, metric[:]...)
		neighborsTLV.valueTLV = append(neighborsTLV.valueTLV, intf.adj.neighborSystemID[:]...)
		glog.V(2).Infof("adding neighbor system id %s", systemIDToString(intf.adj.neighborSystemID[:]))
		neighborsTLV.valueTLV = append(neighborsTLV.valueTLV, pseudoNodeId)
		neighborsTLV.lengthTLV += 11
	}
	return &neighborsTLV
}

func getPrefixesFromTLV(tlv *IsisTLV) []net.IPNet {
    // Given a prefix tlv, return the prefixes as a slice of strings
    prefixes := make([]net.IPNet, 0)
    prefixCount := int(tlv.lengthTLV) / 12 // Each prefix takes up 12 bytes
	glog.V(2).Infof("Prefix count %d in tlv %v", prefixCount, tlv)
	currentPrefix := 0
	for currentPrefix < prefixCount {
        // Skip the metrix
        var currentPrefixValue net.IPNet
        currentPrefixValue.IP =  tlv.valueTLV[currentPrefix*12:currentPrefix*12+4] // Metric is first, skip that
        currentPrefixValue.Mask = tlv.valueTLV[currentPrefix*12 + 4:currentPrefix*12 + 4 + 4]
        glog.V(2).Infof("Current prefix %v", currentPrefixValue)
        prefixes = append(prefixes, currentPrefixValue)
        currentPrefix += 1
    }
    return prefixes 
}

func getDirectlyConnectedPrefixes(systemID string) []net.IPNet{
    // Lookup the lsp and extract the directly connected prefixes
	tmp := AvlSearch(UpdateDB.Root, systemIDToKey(systemID))
	if tmp == nil {
		glog.V(1).Infof("No such LSP %s in LSP database", systemID)
        return nil
	} 
	lsp := tmp.(*IsisLsp)
	currentTLV := lsp.CoreLsp.FirstTLV
	for currentTLV != nil {
		if int(currentTLV.typeTLV) == ISIS_IP_INTERNAL_REACH_TLV {
			return getPrefixesFromTLV(currentTLV)
		}
		currentTLV = currentTLV.nextTLV
	}
	glog.V(2).Infof("No prefix tlv found in LSP %s", systemID)
	return nil
}

func buildEmptyLSP(sequenceNumber uint32, sourceSystemID string) *IsisLsp {
	var newLsp IsisLsp
	newLsp.LspID = systemIDToLspID(sourceSystemID)
	isisPDUHeader := IsisPDUHeader{IntraDomainRouteingProtocolDiscriminator: 0x83,
		LengthPDU:            0x00,
		ProtocolID:           0x01,
		SystemIDLength:       0x00, // 0 means default 6 bytes
		TypePDU:              0x12, // l1 LSP
		Version:              0x01, //
		Reserved:             0x00,
		MaximumAreaAddresses: 0x00} // 0 means default 3 addresses
	var seq [4]byte
	binary.BigEndian.PutUint32(seq[:], sequenceNumber)
	lspHeader := IsisLspHeader{SequenceNumber: seq}
	lspHeader.LspID = newLsp.LspID
	core := IsisLspCore{Header: isisPDUHeader,
		LspHeader: lspHeader,
		FirstTLV:  nil}
	newLsp.CoreLsp = &core
	newLsp.Key = lspIDToKey(newLsp.LspID)
	return &newLsp
}

func generateLocalLsp() {
	// Triggered on adjacency change
	// Build a local LSP from the information in adjacency database
	// Leaving fragment and PSN set to zero for now
	// Sequence number is incremented every time this function is called
	// TODO: See if there is a better way to do this --> probably need to move everything to use byte slices, these fixed arrays are a pain in the ass
	sequenceNumber += 1
	newLsp := buildEmptyLSP(sequenceNumber, cfg.sid)
	// Also include the adjacency tlvs (assuming metric of 10 always)
	reachTLV := getIPReachTLV(cfg.interfaces)
	neighborTLV := getNeighborTLV(cfg.interfaces)
	reachTLV.nextTLV = neighborTLV
	newLsp.CoreLsp.FirstTLV = reachTLV
	UpdateDB.DBLock.Lock()
	UpdateDB.Root = AvlInsert(UpdateDB.Root, newLsp.Key, newLsp, true)
	tmp := AvlSearch(UpdateDB.Root, newLsp.Key)
	UpdateDB.DBLock.Unlock()
	if tmp == nil {
		glog.V(1).Infof("Failed to generate local LSP %s", systemIDToString(newLsp.LspID[:6]))
	} else {
		lsp := tmp.(*IsisLsp)
		glog.V(1).Infof("Successfully generated local LSP %s seq num %d", systemIDToString(lsp.LspID[:6]), sequenceNumber)
	}
	// Lsp has been created, need to flood it on all interfaces
	for _, intf := range cfg.interfaces {
		intf.lock.Lock()
		// Add this LSP to the interfaces flood state
		// If it is already there, just set SRM to true
		if _, inMap := intf.lspFloodStates[newLsp.Key]; !inMap {
			intf.lspFloodStates[newLsp.Key] = &LspFloodState{LspIDKey: newLsp.Key, LspID: newLsp.LspID, SRM: true, SSN: false}
		} else {
			intf.lspFloodStates[newLsp.Key].SRM = true
		}
		intf.lock.Unlock()
	}
}

func printUpdateDB(root *AvlNode) {
	if root != nil {
		printUpdateDB(root.left)
		if root.data != nil {
			glog.V(2).Infof("%v", root.data)
		}
		printUpdateDB(root.right)
	}
}

func printLspFloodStates(intf *Intf) {
	for _, v := range intf.lspFloodStates {
		glog.Infof("%s --> SRM %v", systemIDToString(v.LspID[:6]), v.SRM)
	}
}
