// Ethernet layer
// +build linux

package main

import (
    "bytes"
    "net"
    "syscall"
    "unsafe"
    "encoding/binary"
    "github.com/golang/glog"
)

const (
    PF_PACKET = 17
    ETH_P_ALL = 0x0003
    READ_BUF_SIZE = 100
)

type RawSock struct {
    fd   int
    intf *net.Interface
}

var RawSocks map[string][2]*RawSock // Map of interfaces to send and receive sockets

type IsisPDUHeader struct {
    // Common 8 byte header to all PDUs
    // Note that the fields must be exported for the binary.Read 
    IntraDomainRouteingProtocolDiscriminator byte // 0x83
    LengthPDU byte
    ProtocolID byte
    SystemIDLength byte
    TypePDU byte // first three bits are reserved and set to 0, next 5 bits are pdu type
    Version byte
    Reserved byte
    MaximumAreaAddresses byte
}

type IsisTLV struct {
    nextTLV *IsisTLV
    typeTLV byte
    lengthTLV byte
    valueTLV []byte
}

func htons(host uint16) uint16 {
    return (host & 0xff) << 8 | (host >> 8)
}

func NewRawSock(ifname string) (*RawSock, error) {
    intf, err := net.InterfaceByName(ifname)
    if err != nil {
        return nil, err
    }
    fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
    if err != nil {
        return nil, err
    }
    return &RawSock{
        fd:   fd,
        intf: intf,
    }, nil
}

func NewRawSockRecv(ifname string) (*RawSock, error) {
    intf, err := net.InterfaceByName(ifname)
    if err != nil {
        return nil, err
    }
    fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, int(htons(ETH_P_ALL)))
    if err != nil {
        return nil, err
    }
    sll := syscall.RawSockaddrLinklayer{
        Family:   PF_PACKET,
        Protocol: htons(ETH_P_ALL),
        Ifindex:  int32(intf.Index),
    }
    // Take our socket and bind it
    _, _, e := syscall.Syscall(syscall.SYS_BIND,
                               uintptr(fd),
                               uintptr(unsafe.Pointer(&sll)),
                               unsafe.Sizeof(sll))
    if e > 0 {
        return nil, e
    }
    return &RawSock{
        fd:   fd,
        intf: intf,
    }, nil
}

func (c *RawSock) Read(b []byte) (int, *syscall.RawSockaddrLinklayer, error) {
    var sll syscall.RawSockaddrLinklayer
    size := unsafe.Sizeof(sll)
    r1, _, err := syscall.Syscall6(syscall.SYS_RECVFROM,
                                   uintptr(c.fd),
                                   uintptr(unsafe.Pointer(&b[0])),
                                   uintptr(len(b)),
                                   0,
                                   uintptr(unsafe.Pointer(&sll)),
                                   uintptr(unsafe.Pointer(&size)))
    if err > 0 {
        return 0, nil, err
    }
    return int(r1), &sll, nil
}

func (c *RawSock) Write(b []byte) (n int, err error) {
    // Write a raw ethernet frame to interface in RawSock
    var dst [8]uint8
    for i := 0; i < len(dst); i++ {
        dst[i] = uint8(b[i])
    }
    sll := syscall.RawSockaddrLinklayer{
        Ifindex: int32(c.intf.Index),
        Addr: dst,
        Halen: 6,
    }
    r1, _, e := syscall.Syscall6(syscall.SYS_SENDTO,
                                 uintptr(c.fd),
                                 uintptr(unsafe.Pointer(&b[0])),
                                 uintptr(len(b)),
                                 0,
                                 uintptr(unsafe.Pointer(&sll)),
                                 unsafe.Sizeof(sll))
    if e > 0 {
        return 0, e
    }
    return int(r1), e
}

func getMac(ifname string) []byte {
    intf, _ := net.InterfaceByName(ifname)
    src := make([]byte, len(intf.HardwareAddr))
    copy(src, intf.HardwareAddr)
    return src
}

func buildEthernetFrame(dst []byte, src []byte, payload []byte) []byte {
    // Need a way to put a generic payload in an ethernet frame
    // output needs to be a large byte slice which can be directly sent with Write
    // Ethernet frame needs dst, src, type, payload
    // TODO: figure out how to use encoding/gob here 
    etherType := []byte{0x08, 0x00}
    var buf bytes.Buffer
    // Can't write binary with nil pointer how to handle the TLVs?
    binary.Write(&buf, binary.BigEndian, dst)
    binary.Write(&buf, binary.BigEndian, src)
    binary.Write(&buf, binary.BigEndian, etherType)
    binary.Write(&buf, binary.BigEndian, payload)
    return buf.Bytes()
}

func ethernetInit() {
    RawSocks = make(map[string][2]*RawSock)
} 

func ethernetIntfInit(ifname string) {
    // Create raw send and receive sockets for given interface
    send, err := NewRawSock(ifname)
    if send == nil || err != nil {
        glog.Error("Failed to open raw send socket", err)
    }
    recv, err := NewRawSockRecv(ifname)
    if recv == nil || err != nil {
        glog.Error("Failed to open raw recv socket", err)
    }
    var value [2]*RawSock
    value[0] = send
    value[1] = recv
    RawSocks[ifname] = value
}

func sendFrame(frame []byte, ifname string) {
    // Take in a byte slice payload and send it
    numBytes, e := RawSocks[ifname][0].Write(frame)
    if numBytes <= 0 {
        glog.Error(e.Error())
    }
}

func recvFrame(ifname string) []byte {
    // Only return once a packet has been received which is not one
    // we sent ourselves
    // TODO: tune the buffer size
    src := getMac(ifname)
    var buf bytes.Buffer
    var rawBuf [READ_BUF_SIZE]byte
    buf.Grow(READ_BUF_SIZE)
    for {
        // Blocks until something is available
        buf.Reset() // Zero out the buffer
        numBytes, _, e := RawSocks[ifname][1].Read(rawBuf[:])
        if e != nil {
            glog.Error("Error reading bytes: ", e)
        } else {
            // Return anything that we did not send ourselves
            if ! bytes.Equal(buf.Bytes()[6:12], src) {
                // Now copy numBytes from raw buf into our buffer
                buf.Write(rawBuf[:numBytes])
                return buf.Bytes()
            }
        }
    }
}

func recvPdus(ifname string, hello chan []byte, update chan []byte) {
    // Continuously read from the raw socks associated with the specified
    // interface, putting the packets on the appropriate channels
    // for the other goroutines to process 
    // pdu types:
    //  0x0F --> l1 lan hello
    //  0x12 --> l2 LSP
    for {
        buf := recvFrame(ifname) 
        // TODO: basic checks like length, checksum, auth
        // Check the common IS-IS header for the pdu type
        // This receive frame will have everything including the ethernet frame
        // 14 bytes ethernet header, then its the 5th byte after that in the common header
        // Make sure it is an IS-IS protocol packet
        if buf[14] != 0x83 {
            continue
        }
        pduType := buf[14+4]
        if pduType == 0x0F {
            hello <- buf  
        } else if pduType == 0x12 {
            glog.Infof("Received an LSP %s",  systemIDToString(buf[14+7+5: 14+7+5+6]))
            update <- buf
        }
    }
}

func sendPdus(ifname string, send chan []byte) {
    // Continuously sendPdus 
    for {
        sendFrame(<-send, ifname)
    }
}

