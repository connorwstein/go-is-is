package main

import (
    "fmt"
    "time"
    "bytes"
    "os"
    "os/signal"
    "syscall"
    "sync"
    "encoding/hex"
    "log"
    "net"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    pb "./config"
    "google.golang.org/grpc/reflection"
	"reflect"
)

var wg sync.WaitGroup
var l1_lan_hello_dst []byte
var cfg Config

const (
    GRPC_CFG_SERVER_PORT = ":50051"
)

type Config struct {
    sid string // Format is 6 bytes in a hex encoded string, with a '.' between bytes 2-3 and 4-5
	interfaces []Intf // slice of interfaces
    adjacency Adjacency // Right now only one interface, so just a single adjacency
}

type Intf struct {
	name string
	prefix net.IP
	mask net.IPMask
}

type Adjacency struct {
    intf string // TODO: should be a pointer to the Intf struct
    state string // Can be NEW, INITIALIZING or UP
    neighbor_system_id string 
}


func hello_send() {
    // Send hellos every 2 seconds after a system ID has been configured
	// on all interfaces
    for {
        fmt.Printf("SEND ADJACENCY STATE: %v\n", cfg.adjacency)
        if cfg.sid != "" {
            fmt.Println("Have a sid - sending hello")
            if cfg.adjacency.state != "UP"  {
// 				for _, intf := range cfg.interfaces {
//                 	send_hello(&intf, cfg.sid, nil)
// 				}
				var intf Intf
				intf.name = "eth0"
				send_hello(&intf, cfg.sid, nil)
            }
        }
        // After sending we update the adjacency to NEW
        time.Sleep(4000 * time.Millisecond)
    }
}

func hello_recv() {
    for {
        // Blocks until a hello pdu is received
        hello := recv_hello()
        fmt.Printf("RECV ADJACENCY STATE: %v\n", cfg.adjacency)
        if hello != nil {
            // Depending on what type of hello it is, respond
            // Respond to this hello packet with a IS-Neighbor TLV 
            // If we receive a hello with no neighbor tlv, we copy 
            // the mac of the sender into the neighbor tlv and send it back out
            // then mark the adjacency on that interface as INITIALIZING
            // If we receive a hello with our own mac in the neighbor tlv
            // we mark the adjacency as UP
            fmt.Printf("GOT HELLO FROM %v\n", hello.lan_hello_pdu.source_system_id)
            // even if our adjacency is up, we need to respond to other folks
            if hello.lan_hello_pdu.first_tlv == nil {
                // No TLVs yet in this hello packet so we need to add in the IS neighbors tlv
                // TLV type 6
                // After getting this --> adjacency is in the initializing state
                var neighbors_tlv IsisTLV
                neighbors_tlv.next_tlv = nil
                neighbors_tlv.tlv_type = 6
                neighbors_tlv.tlv_length = 6 // Just one other mac for now
                neighbors_tlv.tlv_value = hello.source_mac // []byte of the senders mac address
                if cfg.adjacency.state != "UP" {
                    fmt.Println("ADJACENCY INIT")
                    cfg.adjacency.state = "INIT"
                }
				var intf Intf
				intf.name = "eth0"
                send_hello(&intf, cfg.sid, &neighbors_tlv)
            } else {
                // If we do have the neighbors tlv, check if it has our own mac in it
                // if it does then we know the adjacency is established
                if bytes.Equal(hello.lan_hello_pdu.first_tlv.tlv_value, get_mac("eth0")) {
                    fmt.Println("ADJACENCY UP")
                    cfg.adjacency.state = "UP"
                    cfg.adjacency.neighbor_system_id = hex.Dump(hello.lan_hello_pdu.source_system_id[:])
                }
            }
        }
    }
}

func cleanup() {
    fmt.Println("cleanup")
}

type server struct{}

func (s *server) ConfigureSystemID(ctx context.Context, in *pb.SystemIDRequest) (*pb.SystemIDReply, error) {
    cfg.sid = in.Sid
    fmt.Println("Got SID request, setting SID to " + cfg.sid)
    // Returning a pointer to the system ID reply struct with a message acknowledging that it was 
    // successfully configured.
    // Note that even through the proto has a the field defined with lowercase, it is converted
    // to uppercase so it can be exported golang style
    return &pb.SystemIDReply{Message: "SID " + in.Sid + " successfully configured"}, nil
}

func (s *server) GetState(ctx context.Context, in *pb.StateRequest) (*pb.StateReply, error) {
    fmt.Println("Got state request, dumping state", cfg)
    interfaces_string := "" // TODO: optimize
    for _, i := range cfg.interfaces {
        interfaces_string += i.prefix.String() + " " + i.mask.String() + ", "
    }
    var reply pb.StateReply
    reply.Intf = interfaces_string 
    if cfg.adjacency.state != "UP" {
        reply.Adj = "Adjacency not yet established"
        return &reply, nil
    } else {
        reply.Adj = "Adjacency established with " + cfg.adjacency.neighbor_system_id
        return &reply, nil
    }
}

func start_grpc() {
    // listening on eth0
    lis, err := net.Listen("tcp", GRPC_CFG_SERVER_PORT)
    if err != nil {
        log.Fatalf("gRPC server failed to start listening: %v", err)
    }
    s := grpc.NewServer()
    pb.RegisterConfigureServer(s, &server{})
    pb.RegisterStateServer(s, &server{})
    // Register reflection service on gRPC server.
    reflection.Register(s)
    if err := s.Serve(lis); err != nil {
        log.Fatalf("gRPC server failed to start serving: %v", err)
    }
}

func getLocalIntfAddresses() {
    ifaces, err := net.Interfaces()
	cfg.interfaces = make([]Intf, len(ifaces) - 1) // Ignore loopback
    fmt.Println(len(ifaces))
	intf_index := 0
    if err != nil {
        fmt.Print(fmt.Errorf("getLocalIntfAddresses: %+v\n", err.Error()))
        return
    }
    for _, i := range ifaces {
        // Ignore loopback interfaces
		if i.Name == "lo" {
			continue
		}	
        addrs, err := i.Addrs()
        if err != nil {
            fmt.Print(fmt.Errorf("getLocalIntfAddresses: %+v\n", err.Error()))
            continue
        }
        for _, a := range addrs {
            switch v := a.(type) {
            case *net.IPNet: //Checking if this type of address a (v) is a pointer to a net.IPNet struct
                fmt.Printf("%v : %s\n", i.Name, v)
                // Only work with v4 addresses for now
                if v.IP.To4() != nil {
                    var new_intf Intf
                    new_intf.name = i.Name
                    new_intf.prefix = v.IP
                    new_intf.mask = v.Mask
                    cfg.interfaces[intf_index] = new_intf
                    intf_index++
                }
			default:
				fmt.Println("Not an ip address", v)
				fmt.Println(reflect.TypeOf(v))
            }

        }
    }
}


func main() {
    // This is a special multicast mac address
    l1_lan_hello_dst = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
    cfg.sid = "" 
    c := make(chan os.Signal, 2)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        cleanup()
        os.Exit(1)
    }()
    // Determine the interfaces available on the container
    // and add that to the configuration
	getLocalIntfAddresses()    
	fmt.Println(cfg.interfaces)

// 	// Start a couple go routines to communicate with other nodes
// 	// to establish adjacencies 
//     // Multicast mac address used in IS-IS hellos
//     wg.Add(1)
//     go hello_send()
//     wg.Add(1)
//     go hello_recv()
    wg.Add(1)
    go start_grpc()
    wg.Wait()
}
