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

func htons(host uint16) uint16 {
    return (host&0xff)<<8 | (host >> 8)
}


// Packet family socket connection, given an interface
func NewPFConn(ifname string) (*PFConn, error) {
    intf, err := net.InterfaceByName(ifname)
    if err != nil {
        return nil, err
    }
    // The ETH_P_ALL is required to get all protocols
    //fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, int(htons(ETH_P_ALL)))
    fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
    if err != nil {
        return nil, err
    }
//     // Set the interface into promiscuous mode
//     mreq := packetMreq{
//         mrIfindex: int32(intf.Index),
//         mrType:    PACKET_MR_PROMISC,
//     }
//     if _, _, e := syscall.Syscall6(syscall.SYS_SETSOCKOPT, uintptr(fd),
//         uintptr(syscall.SOL_PACKET), uintptr(syscall.PACKET_ADD_MEMBERSHIP),
//         uintptr(unsafe.Pointer(&mreq)), unsafe.Sizeof(mreq), 0); e > 0 {
//         return nil, e
//     }
//     sll := syscall.RawSockaddrLinklayer{
//         Family:   PF_PACKET,
//         Protocol: htons(ETH_P_ALL),
//         Ifindex:  int32(intf.Index),
//     }
//     // Take our socket and bind it 
//     if _, _, e := syscall.Syscall(syscall.SYS_BIND, uintptr(fd),
//         uintptr(unsafe.Pointer(&sll)), unsafe.Sizeof(sll)); e > 0 {
//         return nil, e
//     }
    return &PFConn{
        fd:   fd,
        intf: intf,
    }, nil
}

func (c *PFConn) Write(b []byte, dst [8]uint8) (n int, err error) {
    sll := syscall.RawSockaddrLinklayer{
        Ifindex: int32(c.intf.Index),
        Addr: dst,
        Halen: 6, 
    }
	fmt.Println(sll, c.intf.Index)
    fmt.Println(unsafe.Sizeof(sll), len(b))
    r1, _, e := syscall.Syscall6(syscall.SYS_SENDTO, uintptr(c.fd),
        uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)),
        0, uintptr(unsafe.Pointer(&sll)), unsafe.Sizeof(sll))
	fmt.Println(e)
    if e > 0 {
        return 0, e
    }
    return int(r1), e
}

func main() {
	// Start a couple go routines to communicate with other node
	// and exchange routing information
//     fmt.Println("Welcome to the playground!")
// 
//     fmt.Println("The time is", time.Now())
//     ln, _ := net.Listen("tcp", ":8081")
//     conn, _ := ln.Accept()
//     for {
// 		// will listen for message to process ending in newline (\n)
// 		message, _ := bufio.NewReader(conn).ReadString('\n')
// 		// output message received
// 		fmt.Print("Message Received:", string(message))
// 		// sample process for string received
// 		newmessage := strings.ToUpper(message)
// 		// send new string back to client
// 		conn.Write([]byte(newmessage + "\n"))
//     }
	pf, e := NewPFConn("eth0")
	if pf == nil {
		fmt.Printf("Failed to open pf socket", e)
	}
	if e != nil {
		fmt.Println(e)
	}
	//fmt.Println(pf.fd, pf.intf)
    //src 02:42:ac:12:00:02 dst 02:42:ac:12:00:03
    // Src dst and ether type 0800, if any of these are wrong in the ethernet header 
    // then it will fail.
	hello := []byte{0x02, 0x42, 0xac, 0x12, 0x00, 0x03, 0x02, 0x42, 0xac, 0x12, 0x00, 0x02, 0x08, 0x00, 'b', 'e'}
    // For the socket
    dst := [8]uint8{0x02, 0x42, 0xac, 0x12, 0x00, 0x03}
//    struct    ether_header {
 //       u_char    ether_dhost[6];
  //          u_char    ether_shost[6];
   //             u_short    ether_type;
    //            };
 	num_bytes, e := pf.Write(hello, dst)
	if num_bytes <= 0 {
		fmt.Printf(e.Error())
	} else {
		fmt.Println("Wrote", num_bytes, "bytes")
	}
	
}

