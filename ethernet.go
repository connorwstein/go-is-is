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
    READ_BUF_SIZE = 100
)

type PFConn struct {
    fd   int
    intf *net.Interface
}

func htons(host uint16) uint16 {
    // TODO: is this needed?
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

func (c *PFConn) Write(b []byte) (n int, err error) {
    // Write a raw ethernet frame to interface in PFConn
    var dst [8]uint8
    for i := 0; i < len(dst); i++ {
        dst[i] = uint8(b[i]) 
    } 
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

func get_mac(ifname string) []byte {
    intf, _ := net.InterfaceByName(ifname)
    src := make([]byte, len(intf.HardwareAddr))
    copy(src, intf.HardwareAddr)
    return src
}

func send_frame(dst []byte, ifname string) {
    // TODO: Separate this function into some kind of init so we don't open 
    // a new PFConn every time
    pf, e := NewPFConn(ifname)
    if pf == nil || e != nil {
        fmt.Println("Failed to open packet socket", e)
    }
    ether_type := []byte{0x08, 0x00}
    hello_l1_lan := build_l1_iih_pdu([6]byte{0x11, 0x11, 0x11, 0x11, 0x11, 0x11})
    var frame []byte = build_eth_frame(dst, get_mac(ifname), 
                                       ether_type, 
                                       serialize_isis_hello_pdu(hello_l1_lan))
    num_bytes, e := pf.Write(frame)
    if num_bytes <= 0 {
        fmt.Printf(e.Error())
    } 
}

func recv_frame(ifname string) [READ_BUF_SIZE]byte {
    // TODO: fix this hardcoded buffer size
	pf, err := NewPFConnRecv(ifname)
	if pf == nil || err != nil {
		fmt.Printf("Failed to open pf socket for receiving", err)
	}
    src := get_mac(ifname)
    var b [READ_BUF_SIZE]byte
    fmt.Println("Reading from socket...")
    // Only return once a packet has been received which is not ours
    for {
        // Blocks until something is available 
        _, _, e := pf.Read(b[:])
        if e != nil {
            fmt.Println("Error reading bytes: ", e)
        } else {
            // Return anything that we did not send ourselves 
            if ! bytes.Equal(b[6:12], src) {
                return b
            }
        }
    }
}
