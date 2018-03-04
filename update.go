// Receive LSPs and flood them, also
// generate our own LSPs
// The update db should be an AVL tree where the keys are LSP IDs the values contain
// the actual LSP

// Since we are using broadcast links, after an adjacency is formed 
package main
import (
    "time"
//     "fmt"
    "encoding/binary"
    "bytes"
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
    Root *AvlNode 
    // May want to add more information here
}

var LspDB IsisLspDB
// var SRMLsps []uint64 // Just the keys of the LSPs which we need to send

func isisUpdateInput(receiveIntf *Intf, update chan [READ_BUF_SIZE]byte) {
    // TODO: Receive update LSPs and flood them along
    // Need to flood it along to every interface, except the one it came from
    // The one it came from is the one we are listening on
    // This lsp is a raw buffer [READ_BUF_SIZE]byte, need to deserialize
    lsp := <- update
    receivedLsp := deserializeLsp(lsp[:])
    // Check if we already have this LSP, if not, then insert it
    // into our own DB an flood it along to all the other interfaces we have
    // TODO: if we already have a copy and the sequence number is newer, overwrite.
    // if we have a newer copy, send the newer copy back to the source 
    tmp := AvlSearch(LspDB.Root, receivedLsp.Key)
    if tmp == nil {
        // Don't have this LSP
        glog.Infof("Adding new lsp %s to DB", system_id_to_str(receivedLsp.LspID[:6]))
        AvlInsert(LspDB.Root, receivedLsp.Key, receivedLsp)
        // Add this new LSP to all interfaces floodStates, and set SRM to true for all of them EXCEPT this interface which we 
        // received it from
        for _, intf := range cfg.interfaces {
            glog.Info("Receive intf %s send intf %s", receiveIntf.name, intf.name)
            if receiveIntf.name == intf.name {
                intf.lspFloodStates = append(intf.lspFloodStates, &LspFloodState{LspIDKey: receivedLsp.Key, LspID: receivedLsp.LspID, SRM: false, SSN: false})
            } else {
                glog.Infof("Flooding new lsp %s out interface: %s", system_id_to_str(receivedLsp.LspID[:6]), intf.name)
                intf.lspFloodStates = append(intf.lspFloodStates, &LspFloodState{LspIDKey: receivedLsp.Key, LspID: receivedLsp.LspID, SRM: true, SSN: false})
            }
        }
    }
}

func isisUpdate(intf *Intf, send chan []byte) {
    for {
        glog.Info("LSP DB:")
        PrintLspDB(LspDB.Root)
        // Check for SRM == true on this interface, if there
        // then use the key to get the full LSP, send it and clear the flag 
        for _, lspFloodState := range intf.lspFloodStates {
            if lspFloodState.SRM {
                tmp := AvlSearch(LspDB.Root, lspFloodState.LspIDKey)
                if tmp == nil {
                    glog.Errorf("Unable to find %s in lsp db", system_id_to_str(lspFloodState.LspID[:6]))
                } else {
                    lsp := tmp.(*IsisLsp)
                    // Send it out that particular interface
                    send <- buildEthernetFrame(l1_multicast, getMac(intf.name), serializeLsp(lsp.CoreLsp))
                    // No ACK required for LAN interfaces
                    lspFloodState.SRM = false
                }
            }
        }
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
    var coreLsp *IsisLspCore = &IsisLspCore{Header: commonHeader, LspHeader: lspHeader}
    var lsp IsisLsp = IsisLsp{Key: LspIDToKey(lspHeader.LspID), LspID: lspHeader.LspID, CoreLsp: coreLsp}
    return &lsp
}

func serializeLsp(lsp *IsisLspCore) []byte {
    var buf bytes.Buffer
    binary.Write(&buf, binary.BigEndian, lsp.Header)
    binary.Write(&buf, binary.BigEndian, lsp.LspHeader)
    return buf.Bytes()
}


func LspIDToKey(lspID [8]byte) uint64 {
    var key uint64 = binary.BigEndian.Uint64(lspID[:]) // Keyed on the LSP ID's integer value
    return key
}

// func GenerateLocalLsp(intf *Intf, SRM bool) {
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
//     lspHeader = IsisLspHeader{PduLength [2]byte{0x00},
//                               RemainingLifetime [2]byte{0x00, 0x00},
//                               LspID [8]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // System id (6 bytes) + 1 byte PSN + 1 byte fragment 
//                               SequenceNumber [4]byte{0x00, 0x00, 0x00, 0x00},
//                               Checksum: [2]byte{0x00, 0x00},
//                               PAttOLType: 0x00}
    lspHeader := IsisLspHeader{}
    lspHeader.LspID = lspID
    core := IsisLspCore{Header: isisPDUHeader,
                        LspHeader: lspHeader,
                        FirstTlv: nil}
    newLsp.CoreLsp = &core
//     newLsp.Interface = intf
//     newLsp.SRM = SRM
    newLsp.Key =  LspIDToKey(lspID)
    LspDB.Root = AvlInsert(LspDB.Root, newLsp.Key, &newLsp)
//     if SRM {
//         SRMLsps = append(SRMLsps, newLsp.Key)
//     }
    tmp := AvlSearch(LspDB.Root, newLsp.Key)
    if tmp == nil {
        glog.Infof("Failed to generate local LSP %s", system_id_to_str(newLsp.LspID[:6]))
    } else {
        lsp := tmp.(*IsisLsp)
        glog.Infof("Successfully generated local LSP %s", system_id_to_str(lsp.LspID[:6]))
    }
    // Lsp has been created, need to flood it on all interfaces
    for _, intf := range cfg.interfaces {
        // Add this LSP to the interfaces flood state and set SRM to true, I think this might have to be a dictionary
        intf.lspFloodStates = append(intf.lspFloodStates, &LspFloodState{LspIDKey: newLsp.Key, LspID: newLsp.LspID, SRM: true, SSN: false})
    }
}

func PrintLspDB(root *AvlNode) {
    if root != nil {
        PrintLspDB(root.left)
        if root.left == nil && root.right == nil {
            // Leaf node, print it
            if root.data != nil {
                lsp := root.data.(*IsisLsp)
                glog.Infof("%s", system_id_to_str(lsp.LspID[:6]));
            }
        }
        PrintLspDB(root.right)
    }
}
