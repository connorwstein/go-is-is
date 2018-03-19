// Receive LSPs and flood them, also
// generate our own LSPs
// The update db should be an AVL tree where the keys are LSP IDs the values contain
// the actual LSP

// Since we are using broadcast links, after an adjacency is formed 
package main
import (
    "time"
    "encoding/hex"
//     "fmt"
    "unsafe"
	"golang.org/x/sys/unix"
    "github.com/vishvananda/netlink"
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
    Key uint64 // Used for key in the LspDB
    LspID [8]byte
    CoreLsp *IsisLspCore
}

type IsisLspDB struct {
    DBLock sync.Mutex
    Root *AvlNode 
    // May want to add more information here
}

var LspDB *IsisLspDB

func LspDBInit() {
    LspDB = &IsisLspDB{DBLock: sync.Mutex{}, Root: nil}
}

func isisUpdateInput(receiveIntf *Intf, update chan []byte) {
    // TODO: Receive update LSPs and flood them along
    // Need to flood it along to every interface, except the one it came from
    // The one it came from is the one we are listening on
    // This lsp is a raw buffer [READ_BUF_SIZE]byte, need to deserialize
    for {
        lsp := <- update
        receivedLsp := deserializeLsp(lsp[:])
        glog.V(2).Infof("Got lsp update")
        glog.V(2).Infof(hex.Dump(lsp[:]))
        // Check if we already have this LSP, if not, then insert it
        // into our own DB an flood it along to all the other interfaces we have
        // TODO: if we already have a copy and the sequence number is newer, overwrite.
        // if we have a newer copy, send the newer copy back to the source 
        LspDB.DBLock.Lock()
        tmp := AvlSearch(LspDB.Root, receivedLsp.Key)
        if tmp == nil {
            // Don't have this LSP
            glog.Infof("Adding new lsp %s (%v) to DB", system_id_to_str(receivedLsp.LspID[:6]), receivedLsp.Key)
            LspDB.Root = AvlInsert(LspDB.Root, receivedLsp.Key, receivedLsp)
            PrintLspDB(LspDB.Root)
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
        LspDB.DBLock.Unlock()
    }
}

func isisUpdate(intf *Intf, send chan []byte) {
    for {
        glog.V(2).Infof("Locking interface %s", intf.name)
        intf.lock.Lock()
        glog.V(1).Info(intf.lspFloodStates)
        glog.Info("LSP DB:")
        PrintLspDB(LspDB.Root)
        glog.Infof("Intf %s Flood States", intf.name)
        PrintLspFloodStates(intf)
        // Check for SRM == true on this interface, if there
        // then use the key to get the full LSP, send it and clear the flag 
        for _, lspFloodState := range intf.lspFloodStates {
            // Need the adjacency to be UP as well
            if lspFloodState.SRM && intf.adj.state == "UP"{
                tmp := AvlSearch(LspDB.Root, lspFloodState.LspIDKey)
                if tmp == nil {
                    glog.Errorf("Unable to find %s (%v) in lsp db", system_id_to_str(lspFloodState.LspID[:6]), lspFloodState.LspIDKey)
                    glog.Errorf("Lsp DB:")
                    PrintLspDB(LspDB.Root)
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
    var reachTlv IsisTLV 
    ethernetHeaderSize := 14
    tlv_offset := ethernetHeaderSize + int(unsafe.Sizeof(commonHeader)) + int(unsafe.Sizeof(lspHeader))
    if tlv_offset < len(raw_bytes) {
        reachTlv.tlv_type = raw_bytes[tlv_offset]
        reachTlv.tlv_length = raw_bytes[tlv_offset + 1]
        glog.V(2).Infof("TLV code %d received, length %d!\n", raw_bytes[tlv_offset], raw_bytes[tlv_offset + 1])
        reachTlv.tlv_value = make([]byte, reachTlv.tlv_length)
        copy(reachTlv.tlv_value, raw_bytes[tlv_offset + 2: tlv_offset + 2 + int(reachTlv.tlv_length)])
        coreLsp.FirstTlv = &reachTlv
    }
    var lsp IsisLsp = IsisLsp{Key: LspIDToKey(lspHeader.LspID), LspID: lspHeader.LspID, CoreLsp: coreLsp}
    return &lsp
}

func serializeLsp(lsp *IsisLspCore) []byte {
    var buf bytes.Buffer
    binary.Write(&buf, binary.BigEndian, lsp.Header)
    binary.Write(&buf, binary.BigEndian, lsp.LspHeader)
    if lsp.FirstTlv != nil {
        glog.V(2).Info("Serializing ip reach tlv", lsp.FirstTlv)
        binary.Write(&buf, binary.BigEndian, lsp.FirstTlv.tlv_type)
        binary.Write(&buf, binary.BigEndian, lsp.FirstTlv.tlv_length)
        binary.Write(&buf, binary.BigEndian, lsp.FirstTlv.tlv_value)
    }
    return buf.Bytes()
}


func LspIDToKey(lspID [8]byte) uint64 {
    var key uint64 = binary.BigEndian.Uint64(lspID[:]) // Keyed on the LSP ID's integer value
    return key
}

func GenerateLocalLsp() {
    // Triggered on adjacency change
    // Build a local LSP from the information in adjacency database 
    // Leaving fragment and PSN set to zero for now
    var newLsp IsisLsp 
    var lspID [8]byte
    bytes := system_id_to_bytes(cfg.sid)
    copy(lspID[:], bytes[:])
    newLsp.LspID = lspID 
    isisPDUHeader := IsisPDUHeader{Intra_domain_routeing_protocol_discriminator: 0x83,
                                   Pdu_length: 0x00,
                                   Protocol_id: 0x01,
                                   System_id_length: 0x00, // 0 means default 6 bytes
                                   Pdu_type: 0x12, // l1 LSP
                                   Version: 0x01, //
                                   Reserved: 0x00,
                                   Maximum_area_addresses: 0x00} // 0 means default 3 addresses
    lspHeader := IsisLspHeader{}
    lspHeader.LspID = lspID
    core := IsisLspCore{Header: isisPDUHeader,
                        LspHeader: lspHeader,
                        FirstTlv: nil}
    var ipReachTlv IsisTLV;
    ipReachTlv.next_tlv = nil
    ipReachTlv.tlv_type = 128
    ipReachTlv.tlv_length = 0 // Number of directly connected prefixes * 12 bytes 
    // For each interface, need to look up the routes associated
    for _, intf := range cfg.interfaces {
	    link, _ := netlink.LinkByName(intf.name)	
        // Just v4 routes for now, filter by AF_INET
	    routes, _ := netlink.RouteList(link, unix.AF_INET)
        for _, route := range routes {
            // Dst will be nil for loopback
            if route.Dst != nil {
                // Add this route to the TLV
                // 4 bytes metric information
                // 4 bytes for ip prefix
                // 4 bytes for ip subnet mask
                ipReachTlv.tlv_value = append(ipReachTlv.tlv_value, route.Dst.IP[:]...)
                ipReachTlv.tlv_value = append(ipReachTlv.tlv_value, route.Dst.Mask[:]...)
                ipReachTlv.tlv_value = append(ipReachTlv.tlv_value, 10) // Using metric of 10 always (1 hop)
                ipReachTlv.tlv_length += 12 
            }
        }
    }
    core.FirstTlv = &ipReachTlv 
    newLsp.CoreLsp = &core
    newLsp.Key =  LspIDToKey(lspID)
    LspDB.DBLock.Lock()
    LspDB.Root = AvlInsert(LspDB.Root, newLsp.Key, &newLsp)
    tmp := AvlSearch(LspDB.Root, newLsp.Key)
    LspDB.DBLock.Unlock()
    if tmp == nil {
        glog.Infof("Failed to generate local LSP %s", system_id_to_str(newLsp.LspID[:6]))
    } else {
        lsp := tmp.(*IsisLsp)
        glog.Infof("Successfully generated local LSP %s", system_id_to_str(lsp.LspID[:6]))
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

func PrintLspDB(root *AvlNode) {
    if root != nil {
        PrintLspDB(root.left)
        if root.data != nil {
            lsp := root.data.(*IsisLsp)
            glog.Infof("%s", system_id_to_str(lsp.LspID[:6]));
            glog.V(1).Infof("%s -> %v", system_id_to_str(lsp.LspID[:6]), lsp);
            if lsp.CoreLsp.FirstTlv != nil {
                glog.V(1).Infof("\tTLV %d", lsp.CoreLsp.FirstTlv.tlv_type);
            }
        }
        PrintLspDB(root.right)
    }
}

func PrintLspFloodStates(intf *Intf) {
    for _, v := range intf.lspFloodStates {
        glog.Infof("%s --> SRM %v", system_id_to_str(v.LspID[:6]), v.SRM)
    }
}
