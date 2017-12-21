package main

import (
    "fmt"
    "bytes"
    "time"
    "net"
    "os"
    "os/signal"
    "syscall"
	"unsafe"
    "sync"
)

var wg sync.WaitGroup
var l1_lan_hello_dst []byte

type Adjacency struct {
    intf string
    state string // Can be NEW, INITIALIZING or UP
}

const (
    INTRA_DOMAIN_ROUTEING_PROTOCOL_DISCRIMINATOR = 0x83
    PROTOCOL_ID = 0x01
    SYSTEM_ID_LENGTH = 0x06
    L1_LAN_IIH_PDU_TYPE = 0x0F
    VERSION = 0x01
    MAX_AREA_ADDRESSES_DEFAULT = 0x00 // 0 means 3 addresses are supported
)

type IsisPDUHeader struct {
    intra_domain_routeing_protocol_discriminator byte // 0x83
    pdu_length byte
    protocol_id byte
    system_id_length byte
    pdu_type byte // first three bits are reserved and set to 0, next 5 bits are pdu type
    version byte
    maximum_area_addresses byte
}

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
    r1, _, e := syscall.Syscall6(syscall.SYS_RECVFROM, uintptr(c.fd),
        uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
        0, uintptr(unsafe.Pointer(&sll)), uintptr(unsafe.Pointer(&size)))
    if e > 0 {
        return 0, nil, e
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
    for {
        pf, e := NewPFConn(ifname)
        if pf == nil {
            fmt.Printf("Failed to open pf socket", e)
        }
        if e != nil {
            fmt.Println(e)
        }
        intf, _ := net.InterfaceByName(ifname)
        payload := []byte{'h', 'e', 'y'}
        ether_type := []byte{0x08, 0x00}
        src := make([]byte, len(intf.HardwareAddr))
        for j:=0; j<len(intf.HardwareAddr); j++ {
            src[j] = intf.HardwareAddr[j]
        }
        hello := make([]byte, 0) 
        hello = append(hello, dst...)
        hello = append(hello, src...)
        hello = append(hello, ether_type...)
        hello = append(hello, payload...)
        var socket_dst [8]uint8
        for i := 0; i < len(dst); i++ {
            socket_dst[i] = uint8(dst[i]) 
        } 
        // For the socket
        num_bytes, e := pf.Write(hello, socket_dst)
        if num_bytes <= 0 {
            fmt.Printf(e.Error())
        } else {
            fmt.Println("Wrote", num_bytes, "bytes")
        }
        time.Sleep(2000 * time.Millisecond)
    }
}

func recv_frame(ifname string) {
	pf, e := NewPFConnRecv(ifname)
    intf, _ := net.InterfaceByName(ifname)
    src := make([]byte, len(intf.HardwareAddr))
    for j:=0; j<len(intf.HardwareAddr); j++ {
        src[j] = intf.HardwareAddr[j]
    }
	if pf == nil {
		fmt.Printf("Failed to open pf socket for receiving", e)
	}
	if e != nil {
		fmt.Println(e)
	}
    for {
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
}

func cleanup() {
    fmt.Println("cleanup")
}

func main() {
    // Initialize global variable
    l1_lan_hello_dst = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
    c := make(chan os.Signal, 2)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        cleanup()
        os.Exit(1)
    }()
	// Start a couple go routines to communicate with other nodes
	// to establish adjacencies 
    // Multicast mac address used in IS-IS hellos
    wg.Add(1)
    go send_frame(l1_lan_hello_dst, "eth0")
    wg.Add(1)
    go recv_frame("eth0")
    wg.Wait()
}


