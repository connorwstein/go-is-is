// Receive LSPs and flood them, also
// generate our own LSPs
// The update db should be an AVL tree where the keys are LSP IDs the values contain
// the actual LSP

// Since we are using broadcast links, after an adjacency is formed 
package main
import (
    "time"
    "fmt"
    "net"
    "encoding/hex"
    "unsafe"
    "encoding/binary"
    "bytes"
    "sync"
    "github.com/golang/glog"
)

const (
    LSP_REFRESH = 5000
)

type IsisLspHeader struct {
    PduLength [2]byte
    RemainingLifetime [2]byte
    LspID [8]byte // System id (6 bytes) + 1 byte PSN + 1 byte fragment 
    SequenceNumber [4]byte
    Checksum [2]byte
    PAttOLType byte
}

type IsisLspCore struct {
    Header IsisPDUHeader
    LspHeader IsisLspHeader
    FirstTlv *IsisTLV
} 

type IsisLsp struct {
    // Wrapper struct around the core
    Key uint64 // Used for key in the UpdateDB
    LspID [8]byte
    CoreLsp *IsisLspCore
}

func (lsp IsisLsp) String() string {
    var lspString bytes.Buffer
    lspString.WriteString(fmt.Sprintf("%s", system_id_to_str(lsp.LspID[:6])))
    var curr *IsisTLV = lsp.CoreLsp.FirstTlv
    for curr != nil {
        lspString.WriteString(fmt.Sprintf("\tTLV %d\n", curr.tlv_type))
        lspString.WriteString(fmt.Sprintf("\tTLV size %d\n", curr.tlv_length))
        if curr.tlv_type == 128 {
            // This is a external reachability tlv
            // TODO: fix hard coding here
            for i := 0; i < int(curr.tlv_length) / 12; i++ {
                var prefix net.IPNet 
                prefix.IP = curr.tlv_value[i*12:i*12 + 4]
                prefix.Mask = curr.tlv_value[i*12 + 4: i*12 + 8]
                metric := curr.tlv_value[i*12 + 8: i*12 + 12]
                lspString.WriteString(fmt.Sprintf("\t\t%s Metric %d\n", prefix.String(), binary.BigEndian.Uint32(metric[:])))
            }
        } else if curr.tlv_type == 2 {
            // This is a neighbors tlv, its length - 1 (to exclude the first virtualByteFlag) will be a multiple of 11
            for i := 0; i < int(curr.tlv_length - 1)  / 11; i++ {
                // Print out the neighbor system ids and metric 
                metric := curr.tlv_value[i*11 + 4]
                systemID := curr.tlv_value[(i*11 + 1 + 4):(i*11 + 1 + 4 + 6)]
                lspString.WriteString(fmt.Sprintf("\t\t%s Metric %d\n", system_id_to_str(systemID), metric))
            }
        }
        curr = curr.next_tlv
    }            
    return lspString.String() 
}

type IsisDB struct {
    DBLock sync.Mutex
    Root *AvlNode 
    // May want to add more information here
}

type Neighbor struct {
    systemID string
    metric uint32
}

var UpdateDB *IsisDB
var sequenceNumber uint32

func UpdateDBInit() {
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
            if _, inMap := intf.lspFloodStates[receivedLsp.Key]; ! inMap {
                intf.lspFloodStates[receivedLsp.Key] = &LspFloodState{LspIDKey: receivedLsp.Key, LspID: receivedLsp.LspID, SRM: false, SSN: false}
            } else {
                intf.lspFloodStates[receivedLsp.Key].SRM = false 
            }
        } else {
            glog.Infof("Flooding new lsp %s out interface: %s", system_id_to_str(receivedLsp.LspID[:6]), intf.name)
            // If it is already there, just set SRM to true
            if _, inMap := intf.lspFloodStates[receivedLsp.Key]; ! inMap {
                intf.lspFloodStates[receivedLsp.Key] = &LspFloodState{LspIDKey: receivedLsp.Key, LspID: receivedLsp.LspID, SRM: true, SSN: false}
            } else {
                intf.lspFloodStates[receivedLsp.Key].SRM = true 
            }
        }
        glog.V(2).Infof("Unlocking interface %s", intf.name)
        intf.lock.Unlock()
    }
}

func isisUpdateInput(receiveIntf *Intf, update chan []byte) {
    // TODO: Receive update LSPs and flood them along
    // Need to flood it along to every interface, except the one it came from
    // The one it came from is the one we are listening on
    // This lsp is a raw buffer [READ_BUF_SIZE]byte, need to deserialize
    for {
        lsp := <- update
        receivedLsp := deserializeLsp(lsp[:])
        glog.V(2).Infof("Got lsp update %s sequence number %d", system_id_to_str(receivedLsp.LspID[:6]), binary.BigEndian.Uint32(receivedLsp.CoreLsp.LspHeader.SequenceNumber[:]))
        glog.V(2).Infof(hex.Dump(lsp[:]))
        // Check if we already have this LSP, if not, then insert it
        // into our own DB an flood it along to all the other interfaces we have
        // TODO: if we already have a copy and the sequence number is newer, overwrite.
        // if we have a newer copy, send the newer copy back to the source 
        UpdateDB.DBLock.Lock()
        tmp := AvlSearch(UpdateDB.Root, receivedLsp.Key)
        if tmp == nil {
            // Don't have this LSP so lets add it
            glog.Infof("Adding new lsp %s (%v) to DB", system_id_to_str(receivedLsp.LspID[:6]), receivedLsp.Key)
            UpdateDB.Root = AvlInsert(UpdateDB.Root, receivedLsp.Key, receivedLsp, false)
            PrintUpdateDB(UpdateDB.Root)
            floodNewLsp(receiveIntf, receivedLsp)
        } else {
            // We do have this LSP, check if the sequence number is newer than the current version we have if it is then update
            lsp := tmp.(*IsisLsp)
            if binary.BigEndian.Uint32(lsp.CoreLsp.LspHeader.SequenceNumber[:]) < binary.BigEndian.Uint32(receivedLsp.CoreLsp.LspHeader.SequenceNumber[:]) {
                // Received one is newer, update and flood
                glog.Infof("Overwriting new lsp %s (%v) to DB", system_id_to_str(receivedLsp.LspID[:6]), receivedLsp.Key)
                UpdateDB.Root = AvlInsert(UpdateDB.Root, receivedLsp.Key, receivedLsp, true)
                PrintUpdateDB(UpdateDB.Root)
                floodNewLsp(receiveIntf, receivedLsp)
            }
        }
        UpdateDB.DBLock.Unlock()
    }
}

func isisUpdate(intf *Intf, send chan []byte) {
    for {
        glog.V(2).Infof("Locking interface %s", intf.name)
        intf.lock.Lock()
        glog.V(1).Info(intf.lspFloodStates)
        glog.Info("LSP DB:")
        PrintUpdateDB(UpdateDB.Root)
        glog.Infof("Intf %s Flood States", intf.name)
        PrintLspFloodStates(intf)
        // Check for SRM == true on this interface, if there
        // then use the key to get the full LSP, send it and clear the flag 
        for _, lspFloodState := range intf.lspFloodStates {
            // Need the adjacency to be UP as well
            if lspFloodState.SRM && intf.adj.state == "UP"{
                tmp := AvlSearch(UpdateDB.Root, lspFloodState.LspIDKey)
                if tmp == nil {
                    glog.Errorf("Unable to find %s (%v) in lsp db", system_id_to_str(lspFloodState.LspID[:6]), lspFloodState.LspIDKey)
                    glog.Errorf("Lsp DB:")
                    PrintUpdateDB(UpdateDB.Root)
                } else {
                    lsp := tmp.(*IsisLsp)
                    // Send it out that particular interface
                    glog.Infof("Flooding %s out %s", system_id_to_str(lspFloodState.LspID[:6]), intf.name)
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
    var coreLsp *IsisLspCore = &IsisLspCore{Header: commonHeader, LspHeader: lspHeader, FirstTlv: nil}
    ethernetHeaderSize := 14
    tlv_offset := ethernetHeaderSize + int(unsafe.Sizeof(commonHeader)) + int(unsafe.Sizeof(lspHeader))
    // Check if the tlv offset is strictly less than the raw bytes, if it is then there must be TLVs present
    // keep reading until remaining tlv data is 0, building up a linked list of the TLVs as we go
    remainingTlvBytes := len(raw_bytes) - tlv_offset
    var curr *IsisTLV
    first := true
    for remainingTlvBytes > 0 {
        var currentTlv IsisTLV 
        currentTlv.tlv_type = raw_bytes[tlv_offset]
        currentTlv.tlv_length = raw_bytes[tlv_offset + 1]
        glog.V(2).Infof("TLV code %d received, length %d!\n", raw_bytes[tlv_offset], raw_bytes[tlv_offset + 1])
        currentTlv.tlv_value = make([]byte, currentTlv.tlv_length)
        copy(currentTlv.tlv_value, raw_bytes[tlv_offset + 2: tlv_offset + 2 + int(currentTlv.tlv_length)])
        remainingTlvBytes -= (int(currentTlv.tlv_length) + 2) // + 2 for type and length
        tlv_offset += int(currentTlv.tlv_length) + 2
        if first {
            coreLsp.FirstTlv = &currentTlv
            curr = coreLsp.FirstTlv
            first = false
        } else {
            curr.next_tlv = &currentTlv
            curr = curr.next_tlv
        }
    }
    var lsp IsisLsp = IsisLsp{Key: LspIDToKey(lspHeader.LspID), LspID: lspHeader.LspID, CoreLsp: coreLsp}
    return &lsp
}

func serializeLsp(lsp *IsisLspCore) []byte {
    var buf bytes.Buffer
    binary.Write(&buf, binary.BigEndian, lsp.Header)
    binary.Write(&buf, binary.BigEndian, lsp.LspHeader)
    tlv := lsp.FirstTlv
    for tlv != nil {
        glog.V(2).Info("Serializing tlv:", tlv.tlv_type)
        binary.Write(&buf, binary.BigEndian, tlv.tlv_type)
        binary.Write(&buf, binary.BigEndian, tlv.tlv_length)
        binary.Write(&buf, binary.BigEndian, tlv.tlv_value)
        tlv = tlv.next_tlv
    } 
    return buf.Bytes()
}

func SystemIDToKey(systemID string) uint64 {
    var lspID [8]byte
    bytes := system_id_to_bytes(systemID)
    copy(lspID[:], bytes[:])
    return LspIDToKey(lspID)
}

func LspIDToKey(lspID [8]byte) uint64 {
    var key uint64 = binary.BigEndian.Uint64(lspID[:]) // Keyed on the LSP ID's integer value
    return key
}

func SystemIDToLspID(systemID string) [8]byte {
    var lspID [8]byte
    bytes := system_id_to_bytes(systemID)
    copy(lspID[:], bytes[:])
    return lspID
}

func getNeighbors(neighborTLV *IsisTLV) []*Neighbor { 
    var neighbors []*Neighbor
    neighborCount := (int(neighborTLV.tlv_length) - 1) / 11
    glog.V(2).Infof("Neighbor count %d", neighborCount)
    neighbors = make([]*Neighbor, neighborCount)
    currentNeighbor := 0
    currentByte := 1 // Skip first virtual byte
    for currentNeighbor < neighborCount {
        // Tlv value is 1 virtual byte flag and then n multiples of 4 byte metric and 6 byte system id + 1 byte pseudo-node id
        // Set pseudo-node id to 0 for now
        var neighbor Neighbor
        neighbor.metric = binary.BigEndian.Uint32(neighborTLV.tlv_value[currentByte:currentByte + 4])
        currentByte += 4
        neighbor.systemID = system_id_to_str(neighborTLV.tlv_value[currentByte:currentByte + 6])
        currentByte += 7 // Skip PSN
        glog.V(2).Infof("Current byte %d", currentByte)
        neighbors[currentNeighbor] = &neighbor
        currentNeighbor++
    }
    return neighbors
}

func lookupNeighbors(lsp *IsisLsp) []*Neighbor{
    // Given an LSP returns  list of neig
    //var neighbors []*Neighbor
    currentTLV := lsp.CoreLsp.FirstTlv
    for currentTLV != nil {
        if int(currentTLV.tlv_type) == 2 {
            return getNeighbors(currentTLV)
        }
        currentTLV = currentTLV.next_tlv
    }
    glog.V(2).Infof("No neighbor tlv found in LSP %s", system_id_to_str(lsp.LspID[:6]))
    return nil
}

func getIPReachTLV(interfaces []*Intf) *IsisTLV {
    // Doesn't handle duplicate prefixes reachable via different interfaces
    var ipReachTlv IsisTLV;
    ipReachTlv.next_tlv = nil
    ipReachTlv.tlv_type = 128
    ipReachTlv.tlv_length = 0 // Number of directly connected prefixes * 12 bytes 
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
                ipReachTlv.tlv_value = append(ipReachTlv.tlv_value, route.IP[:]...)
                ipReachTlv.tlv_value = append(ipReachTlv.tlv_value, route.Mask[:]...)
                metric := [4]byte{0x00, 0x00, 0x00, 0x0a}
                ipReachTlv.tlv_value = append(ipReachTlv.tlv_value, metric[:]...) // Using metric of 10 always (1 hop)
                ipReachTlv.tlv_length += 12 
            }
        }
    }
    return &ipReachTlv
}


func getNeighborTLV(interfaces []*Intf) *IsisTLV {
    var neighborsTlv IsisTLV;
    neighborsTlv.next_tlv = nil
    neighborsTlv.tlv_type = 2
    neighborsTlv.tlv_length = 1  // Start at 1 to include virtual byte flag
    var virtualByteFlag byte = 0x00;
    var pseudoNodeId byte = 0x00;
    neighborsTlv.tlv_value = append(neighborsTlv.tlv_value, virtualByteFlag)
    // Loop though the interfaces looking for adjacencies to append
    for _, intf := range interfaces {
        // Tlv value is 1 virtual byte flag and then n multiples of 4 byte metric and 6 byte system id + 1 byte pseudo-node id
        // Set pseudo-node id to 0 for now
        if intf.adj.state != "UP" {
            // Only send the adjacencies that we actually have
            continue
        }
        metric := [4]byte{0x00, 0x00, 0x00, 0x0a}
        neighborsTlv.tlv_value = append(neighborsTlv.tlv_value, metric[:]...)
        neighborsTlv.tlv_value = append(neighborsTlv.tlv_value, intf.adj.neighbor_system_id[:]...)
        glog.Infof("adding neighbor system id %s", system_id_to_str(intf.adj.neighbor_system_id[:]))
        neighborsTlv.tlv_value = append(neighborsTlv.tlv_value, pseudoNodeId)
        neighborsTlv.tlv_length += 11 
    }
    return &neighborsTlv
}

func buildEmptyLSP(sequenceNumber uint32, sourceSystemID string) *IsisLsp {
    var newLsp IsisLsp 
    newLsp.LspID = SystemIDToLspID(sourceSystemID)
    isisPDUHeader := IsisPDUHeader{Intra_domain_routeing_protocol_discriminator: 0x83,
                                   Pdu_length: 0x00,
                                   Protocol_id: 0x01,
                                   System_id_length: 0x00, // 0 means default 6 bytes
                                   Pdu_type: 0x12, // l1 LSP
                                   Version: 0x01, //
                                   Reserved: 0x00,
                                   Maximum_area_addresses: 0x00} // 0 means default 3 addresses
    var seq [4]byte
    binary.BigEndian.PutUint32(seq[:], sequenceNumber)
    lspHeader := IsisLspHeader{SequenceNumber: seq}
    lspHeader.LspID = newLsp.LspID
    core := IsisLspCore{Header: isisPDUHeader,
                        LspHeader: lspHeader,
                        FirstTlv: nil}
    newLsp.CoreLsp = &core
    newLsp.Key =  LspIDToKey(newLsp.LspID)
    return &newLsp
}

func GenerateLocalLsp() {
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
    reachTLV.next_tlv = neighborTLV
    newLsp.CoreLsp.FirstTlv = reachTLV 
    UpdateDB.DBLock.Lock()
    UpdateDB.Root = AvlInsert(UpdateDB.Root, newLsp.Key, newLsp, true)
    tmp := AvlSearch(UpdateDB.Root, newLsp.Key)
    UpdateDB.DBLock.Unlock()
    if tmp == nil {
        glog.Infof("Failed to generate local LSP %s", system_id_to_str(newLsp.LspID[:6]))
    } else {
        lsp := tmp.(*IsisLsp)
        glog.Infof("Successfully generated local LSP %s seq num %d", system_id_to_str(lsp.LspID[:6]), sequenceNumber)
    }
    // Lsp has been created, need to flood it on all interfaces
    for _, intf := range cfg.interfaces {
        intf.lock.Lock()
        // Add this LSP to the interfaces flood state 
        // If it is already there, just set SRM to true
        if _, inMap := intf.lspFloodStates[newLsp.Key]; ! inMap {
            intf.lspFloodStates[newLsp.Key] = &LspFloodState{LspIDKey: newLsp.Key, LspID: newLsp.LspID, SRM: true, SSN: false}
        } else {
            intf.lspFloodStates[newLsp.Key].SRM = true 
        }
        intf.lock.Unlock()
    }
}

func PrintUpdateDB(root *AvlNode) {
    if root != nil {
        PrintUpdateDB(root.left)
        if root.data != nil {
            glog.V(2).Infof("%v", root.data)
        }
        PrintUpdateDB(root.right)
    }
}

func PrintLspFloodStates(intf *Intf) {
    for _, v := range intf.lspFloodStates {
        glog.Infof("%s --> SRM %v", system_id_to_str(v.LspID[:6]), v.SRM)
    }
}
