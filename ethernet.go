// Functions to send and receive ethernet frames
package main

import (
    "fmt"
    "bytes"
    "net"
    "syscall"
	"unsafe"
)

const (
	PF_PACKET = 17
    ETH_P_ALL = 0x0003
)

type PFConn struct {
    fd   int
    intf *net.Interface
}

func htons(host uint16) uint16 {
    return (host&0xff)<<8 | (host >> 8)
}

func NewPFConn(ifname string) (*PFConn, error) {
    intf, err := net.InterfaceByName(ifname)
    if err != nil {
        return nil, err
    }
    fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
    if err != nil {
        return nil, err
    }
    return &PFConn{
        fd:   fd,
        intf: intf,
    }, nil
}

func NewPFConnRecv(ifname string) (*PFConn, error) {
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
    if _, _, e := syscall.Syscall(syscall.SYS_BIND, uintptr(fd),
        uintptr(unsafe.Pointer(&sll)), unsafe.Sizeof(sll)); e > 0 {
        return nil, e
    }
    return &PFConn{
        fd:   fd,
        intf: intf,
    }, nil
}

func (c *PFConn) Read(b []byte) (int, *syscall.RawSockaddrLinklayer, error) {
    var sll syscall.RawSockaddrLinklayer
    size := unsafe.Sizeof(sll)
    r1, _, err := syscall.Syscall6(syscall.SYS_RECVFROM, uintptr(c.fd),
                                 uintptr(unsafe.Pointer(&b[0])), 
                                 uintptr(len(b)),
                                 0, uintptr(unsafe.Pointer(&sll)), 
                                 uintptr(unsafe.Pointer(&size)))
    if err > 0 {
        return 0, nil, err
    }
    return int(r1), &sll, nil
}

func (c *PFConn) Write(b []byte, dst [8]uint8) (n int, err error) {
    sll := syscall.RawSockaddrLinklayer{
        Ifindex: int32(c.intf.Index),
        Addr: dst,
        Halen: 6, 
    }
    r1, _, e := syscall.Syscall6(syscall.SYS_SENDTO, uintptr(c.fd),
        uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
        0, uintptr(unsafe.Pointer(&sll)), unsafe.Sizeof(sll))
    if e > 0 {
        return 0, e
    }
    return int(r1), e
}

func send_frame(dst []byte, ifname string) {
    pf, e := NewPFConn(ifname)
    if pf == nil {
        fmt.Printf("Failed to open pf socket", e)
    }
    if e != nil {
        fmt.Println(e)
    }
    intf, _ := net.InterfaceByName(ifname)
    ether_type := []byte{0x08, 0x00}
    src := make([]byte, len(intf.HardwareAddr))
    for j:=0; j<len(intf.HardwareAddr); j++ {
        src[j] = intf.HardwareAddr[j]
    }
    hello_l1_lan := build_l1_iih_pdu([6]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11})
    var frame []byte = build_eth_frame(dst, src, ether_type, serialize_isis_hello_pdu(hello_l1_lan))
    var socket_dst [8]uint8
    for i := 0; i < len(dst); i++ {
        socket_dst[i] = uint8(dst[i]) 
    } 
    // For the socket
    num_bytes, e := pf.Write(frame, socket_dst)
    if num_bytes <= 0 {
        fmt.Printf(e.Error())
    } else {
        fmt.Println("Wrote", num_bytes, "bytes")
    }
}

func recv_frame(ifname string) {
	pf, err := NewPFConnRecv(ifname)
	if pf == nil {
		fmt.Printf("Failed to open pf socket for receiving", err)
	}
	if err != nil {
		fmt.Println(err)
	}
    intf, _ := net.InterfaceByName(ifname)
    src := make([]byte, len(intf.HardwareAddr))
    for j:=0; j<len(intf.HardwareAddr); j++ {
        src[j] = intf.HardwareAddr[j]
    }
    var b [30]byte
    fmt.Println("Waiting for packet")
    // Blocks until something is available with destination l2_lan_hello_dst
    _, _, e := pf.Read(b[:])
    // Look for packets that are not our own
    if ! bytes.Equal(b[6:12], src) {
        fmt.Printf("Got real packet from: ")
        fmt.Printf("%X:%X:%X:%X:%X:%X\n", b[6], b[7], b[8], b[9], b[10], b[11])
        fmt.Println(string(b[14:]))
        // Respond to this hello packet with a IS-Neighbor TLV 
    }
    if e != nil {
        fmt.Println("Error reading bytes: ", e)
    }
}
