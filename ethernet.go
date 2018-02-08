// Functions to send and receive ethernet frames
// Should be agnostic to the type of data being sent
// Lowest layer
package main

import (
//     "fmt"
    "bytes"
    "net"
    "syscall"
	"unsafe"
    "encoding/binary"
    "github.com/golang/glog"
//     "encoding/hex"
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

var RawSocks []RawSock // An array of raw sockets, one per interface

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

func get_mac(ifname string) []byte {
    intf, _ := net.InterfaceByName(ifname)
    src := make([]byte, len(intf.HardwareAddr))
    copy(src, intf.HardwareAddr)
    return src
}

func build_eth_frame(dst []byte, src []byte, payload []byte) []byte {
    // Need a way to put a generic payload in an ethernet frame
    // output needs to be a large byte slice which can be directly sent with Write
    // Ethernet frame needs dst, src, type, payload
    ether_type := []byte{0x08, 0x00}
    var buf bytes.Buffer
    // Can't write binary with nil pointer how to handle the TLVs?
    binary.Write(&buf, binary.BigEndian, dst)
    binary.Write(&buf, binary.BigEndian, src)
    binary.Write(&buf, binary.BigEndian, ether_type)
    binary.Write(&buf, binary.BigEndian, payload)
//     fmt.Println(hex.Dump(buf.Bytes()))
    return buf.Bytes()
}

func send_frame(frame []byte, ifname string) {
    // Take in a byte slice payload and send it
    // TODO: Separate this function into some kind of init so we don't open
    // a new RawSock every time
    pf, e := NewRawSock(ifname)
    if pf == nil || e != nil {
        glog.Error("Failed to open packet socket", e)
    }
    num_bytes, e := pf.Write(frame)
    if num_bytes <= 0 {
        glog.Error(e.Error())
    }
}

func recv_frame(ifname string) [READ_BUF_SIZE]byte {
    // TODO: tune the buffer size
    // keep the socket open
	pf, err := NewRawSockRecv(ifname)
	if pf == nil || err != nil {
		glog.Error("Failed to open pf socket for receiving", err)
	}
    src := get_mac(ifname)
    var b [READ_BUF_SIZE]byte
    // Only return once a packet has been received which is not ours
    for {
        // Blocks until something is available
        _, _, e := pf.Read(b[:])
        if e != nil {
            glog.Error("Error reading bytes: ", e)
        } else {
            // Return anything that we did not send ourselves
            if ! bytes.Equal(b[6:12], src) {
                return b
            }
        }
    }
}
