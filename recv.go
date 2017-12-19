package main

type packetMreq struct {
    mrIfindex int32
    mrType    uint16
    mrAlen    uint16
    mrAddress [8]uint8
}

func NewPFConnRecv(ifname string) (*PFConn, error) {
    intf, err := net.InterfaceByName(ifname)
    if err != nil {
        return nil, err
    }
    // The ETH_P_ALL is required to get all protocols
    fd, err := syscall.Socket(PF_PACKET, syscall.SOCK_RAW, int(htons(ETH_P_ALL)))
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

