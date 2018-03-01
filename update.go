// Receive LSPs and flood them, also
// generate our own LSPs
// The update db should be an AVL tree where the keys are LSP IDs the values contain
// the actual LSP, let's use just a slice for now
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
    LspID [8]byte
    Interface *Intf
    SRM bool //If this is set, the LSP will get sent
    CoreLsp *IsisLspCore
}

var lsps [100]*IsisLsp // TODO: use AVL tree

func isisUpdateInput(intf *Intf, update chan [READ_BUF_SIZE]byte, send chan []byte) {
    // TODO: Receive update LSPs and flood them along
    // Need to flood it along to every interface, except the one it came from
    // The one it came from is the one we are listening on
    lsp := <- update
    glog.Info("Received an LSP", lsp)
    // We now need the AVL tree 
    
}

func isisUpdate(send chan []byte) {
    // Send out LSP updates via intf
    // Put them on the send channel
    // send_frame(payload, intf.name)
    for {
        // Periodically refresh the LSPs 
        // Any LSPs with their SRM flag set, we send
        i := 0
        currLsp := lsps[i]
        for currLsp != nil {
            if currLsp.SRM {
                // Senddddd it bro
                glog.Info("Sending local lsp", currLsp)
                send <- buildEthernetFrame(l1_multicast, getMac(currLsp.Interface.name), serializeLsp(currLsp.CoreLsp))
            }
            i += 1
            currLsp = lsps[i]
        }
        time.Sleep(LSP_REFRESH * time.Millisecond)
    }
}

func serializeLsp(lsp *IsisLspCore) []byte {
    var buf bytes.Buffer
    binary.Write(&buf, binary.BigEndian, lsp.Header)
    binary.Write(&buf, binary.BigEndian, lsp.LspHeader)
    return buf.Bytes()
}

func GenerateLocalLsp(intf *Intf, SRM bool) {
    // Intf will be required for prefix TLVs
    // Create a local lsp based on the information stored in the adjacency database
    // Changes the adjacency information triggers this
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
    newLsp.Interface = intf
    newLsp.SRM = SRM
    i := 0
    currLsp := lsps[i]
    for currLsp != nil {
        i += 1
        currLsp = lsps[i]
    }
    lsps[i] = &newLsp
    glog.Info("Generated local lsp", lsps[i])
}
