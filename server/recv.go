package main

import (
    "fmt"
//    "time"
    "net"
//   "bufio"
//    "strings"
    "syscall"
	"unsafe"
)

const (
	PF_PACKET = 17
    PACKET_BROADCAST  = 1      
    PACKET_MR_PROMISC = 1      
    ETH_P_ALL         = 0x0003
)

func htons(host uint16) uint16 {
    return (host&0xff)<<8 | (host >> 8)
}

type packetMreq struct {
    mrIfindex int32
    mrType    uint16
    mrAlen    uint16
    mrAddress [8]uint8
}

type PFConn struct {
    fd   int
    intf *net.Interface
}

func (c *PFConn) read(b []byte) (int, *syscall.RawSockaddrLinklayer, error) {
    var sll syscall.RawSockaddrLinklayer
    size := unsafe.Sizeof(sll)
	fmt.Println("Calling read!")
    r1, _, e := syscall.Syscall6(syscall.SYS_RECVFROM, uintptr(c.fd),
        uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
        0, uintptr(unsafe.Pointer(&sll)), uintptr(unsafe.Pointer(&size)))
	fmt.Println("Read returns", r1, e, b)
    if e > 0 {
        return 0, nil, e
    }
    return int(r1), &sll, nil
}

func (c *PFConn) Read(b []byte) (int, error) {
    for {
        n, from, err := c.read(b)
		fmt.Println("Got packet ", n, from)
        if err != nil {
            return 0, err
        }
        if from.Pkttype == PACKET_BROADCAST {
            return n, nil
        }
    }
}

func NewPFConnRecv(ifname string) (*PFConn, error) {
    intf, err := net.InterfaceByName(ifname)
    if err != nil {
        return nil, err
    }
    // The ETH_P_ALL is required to get all protocols
    fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, int(htons(ETH_P_ALL)))
    //fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
    if err != nil {
        return nil, err
    }
     // Set the interface into promiscuous mode
//     mreq := packetMreq{
//         mrIfindex: int32(intf.Index),
//         mrType:    PACKET_MR_PROMISC,
//     }
//     if _, _, e := syscall.Syscall6(syscall.SYS_SETSOCKOPT, uintptr(fd),
//       uintptr(syscall.SOL_PACKET), uintptr(syscall.PACKET_ADD_MEMBERSHIP),
//         uintptr(unsafe.Pointer(&mreq)), unsafe.Sizeof(mreq), 0); e > 0 {
//         return nil, e
//     }
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

func main() {
	pf, e := NewPFConnRecv("eth0")
	if pf == nil {
		fmt.Printf("Failed to open pf socket for receiving", e)
	}
	if e != nil {
		fmt.Println(e)
	}
    for {
		var b [100]byte
    	fmt.Printf("Waiting for packet")
		n, e := pf.Read(b[:])
		fmt.Printf("Results ", string(n), e, b[:])
		if n > 0 {
			fmt.Println("Read bytes: ", b)
		} else {
			fmt.Println("Error reading bytes: ", e)
		}
    }
}
