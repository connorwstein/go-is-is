package main

import (
    "flag"
    "fmt"
    "time"
    "bytes"
    "os"
    "os/signal"
    "syscall"
    "sync"
    "encoding/hex"
//     "log"
    "net"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    pb "./config"
    "google.golang.org/grpc/reflection"
// 	"reflect"
    "github.com/golang/glog"
)

var wg sync.WaitGroup
var l1_lan_hello_dst []byte
var cfg Config

const (
    GRPC_CFG_SERVER_PORT = ":50051"
    RECV_LOG_PREFIX = "RECV:"
    SEND_LOG_PREFIX = "SEND:"
    HELLO_INTERVAL = 4000 // Milliseconds in between hello udpates
)

type Config struct {
    sid string // Format is 6 bytes in a hex encoded string, with a '.' between bytes 2-3 and 4-5
    // Keep adjacencies and interfaces separate in case we want to do multiple
    // IS-IS levels, in which case there would be a level-1 and level-2 adjacency
    // each pointing to the same interface
	interfaces []*Intf // Slice of local interfaces
}

type Intf struct {
    adj *Adjacency
	name string
	prefix net.IP
	mask net.IPMask
    listening bool
}

type Adjacency struct {
    state string // Can be NEW, INITIALIZING or UP
    neighbor_system_id string
}

func hello_send(intf *Intf) {
    // Send hellos every HELLO_INTERVAL after a system ID has been configured
	// on the specified interface
    for {
        if cfg.sid != "" {
            glog.Infof("Adjacency state on %v: %v", intf.name, intf.adj.state)
            if intf.adj.state != "UP" {
                send_hello(intf, cfg.sid, nil)
            }
        }
        time.Sleep(HELLO_INTERVAL * time.Millisecond)
    }
}

func hello_recv(intf *Intf) {
    // Forever receiving hellos on the passed interface
    // Updating the status of the interface as an adjacency is
    // established
    for {
        glog.Info(RECV_LOG_PREFIX, "Receving on intf: ", intf.name)
        rsp := recv_hello(intf)
        // Can get a nil response for ethernet frames received
        // which are not destined for the IS-IS hello multicast address
        if rsp == nil {
            continue
        }
        glog.Info(RECV_LOG_PREFIX, "%v: %v\n", rsp, rsp.intf)
        // Depending on what type of hello it is, respond
        // Respond to this hello packet with a IS-Neighbor TLV
        // If we receive a hello with no neighbor tlv, we copy
        // the mac of the sender into the neighbor tlv and send it back out
        // then mark the adjacency on that interface as INITIALIZING
        // If we receive a hello with our own mac in the neighbor tlv
        // we mark the adjacency as UP
        glog.Infof("Got hello from %v\n", rsp.lan_hello_pdu.source_system_id)
        // even if our adjacency is up, we need to respond to other folks
        if rsp.lan_hello_pdu.first_tlv == nil {
            // No TLVs yet in this hello packet so we need to add in the IS neighbors tlv
            // TLV type 6
            // After getting this --> adjacency is in the initializing state
            var neighbors_tlv IsisTLV
            neighbors_tlv.next_tlv = nil
            neighbors_tlv.tlv_type = 6
            neighbors_tlv.tlv_length = 6 // Just one other mac for now
            neighbors_tlv.tlv_value = rsp.source_mac // []byte of the senders mac address
            if rsp.intf.adj.state != "UP" {
                glog.Infof("Initializing adjacency on intf %v", intf.name)
                rsp.intf.adj.state = "INIT"
            }
            // Send a hello back out the interface we got the response on
            // But with the neighbor tlv
            send_hello(rsp.intf, cfg.sid, &neighbors_tlv)
        } else {
            // If we do have the neighbors tlv, check if it has our own mac in it
            // if it does then we know the adjacency is established
            if bytes.Equal(rsp.lan_hello_pdu.first_tlv.tlv_value, get_mac(rsp.intf.name)) {
                rsp.intf.adj.state = "UP"
                rsp.intf.adj.neighbor_system_id = hex.Dump(rsp.lan_hello_pdu.source_system_id[:])
                glog.Infof("Adjacency up between %v and %v on intf %v", cfg.sid, rsp.intf.adj.neighbor_system_id, intf.name)
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
    glog.Info("Got SID request, setting SID to " + cfg.sid)
    // Returning a pointer to the system ID reply struct with a message acknowledging that it was
    // successfully configured.
    // Note that even through the proto has a the field defined with lowercase, it is converted
    // to uppercase so it can be exported golang style
    return &pb.SystemIDReply{Message: "SID " + in.Sid + " successfully configured"}, nil
}

func (s *server) GetState(ctx context.Context, in *pb.StateRequest) (*pb.StateReply, error) {
    glog.Info("Got state request, dumping state", cfg)
    interfaces_string := "" // TODO: optimize
    for _, i := range cfg.interfaces {
        if i.adj.state != "UP" {
            interfaces_string += i.prefix.String() + " " + i.mask.String() + ", adjacency " + i.adj.state
        } else {
            interfaces_string += i.prefix.String() + " " + i.mask.String() + ", adjacency " + i.adj.state + " with " + i.adj.neighbor_system_id
        }
    }
    var reply pb.StateReply
    reply.Intf = interfaces_string
    return &reply, nil
}

func start_grpc() {
    lis, err := net.Listen("tcp", GRPC_CFG_SERVER_PORT)
    if err != nil {
        glog.Fatalf("gRPC server failed to start listening: %v", err)
    }
    s := grpc.NewServer()
    pb.RegisterConfigureServer(s, &server{})
    pb.RegisterStateServer(s, &server{})
    // Register reflection service on gRPC server.
    reflection.Register(s)
    if err := s.Serve(lis); err != nil {
        glog.Fatalf("gRPC server failed to start serving: %v", err)
    }
}

func initInterfaces() {
    // Initialize the configuration of this IS-IS node
    // with the interface information and a NEW adjacency per
    // interface.
    ifaces, err := net.Interfaces()
	cfg.interfaces = make([]*Intf, len(ifaces) - 1)
	index := 0
    if err != nil {
        glog.Errorf("initInterfaces: %+v\n", err.Error())
        return
    }

    for _, i := range ifaces {
        // Ignore loopback interfaces
		if i.Name == "lo" {
			continue
		}
        addrs, err := i.Addrs()
        if err != nil {
            glog.Errorf("initInterfaces: %+v\n", err.Error())
            continue
        }
        for _, a := range addrs {
            switch v := a.(type) {
            case *net.IPNet: // Checking if this type of address a (v) is a pointer to a net.IPNet struct
                glog.Info("Found interface ", i.Name, ": ",  v)
                // Only work with v4 addresses for now
                if v.IP.To4() != nil {
                    var new_intf Intf
                    new_intf.name = i.Name
                    new_intf.prefix = v.IP
                    new_intf.mask = v.Mask
                    var adj Adjacency
                    adj.state = "NEW"
                    new_intf.adj = &adj
                    cfg.interfaces[index] = &new_intf
                    index++
                } else {
                    // TODO: ipv6 support
                    glog.Info("IPV6 interface ", i.Name, " not supported")
                }
			default:
				glog.Errorf("Not an ip address %+v\n", v)
            }

        }
    }
}


func main() {
    flag.Parse()
    glog.Info("Booting IS-IS node...")

    // This is a special multicast mac address
    l1_lan_hello_dst = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
    cfg.sid = ""

    // Exit go routine
    c := make(chan os.Signal, 2)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        cleanup()
        os.Exit(1)
    }()

    // Determine the interfaces available on the container
    // and add that to the configuration
	initInterfaces()

	// Start a couple go routines to communicate with other nodes
	// to establish adjacencies. Each go routine can run
    // totally in parallel to establish adjacencies on each
    // interface
    wg.Add(1)
    for _, intf := range cfg.interfaces {
        go hello_send(intf)
    }
    wg.Add(1)
    for _, intf := range cfg.interfaces {
        go hello_recv(intf)
    }
    // Start the gRPC server for accepting configuration (CLI commands)
    wg.Add(1)
    go start_grpc()
    wg.Wait()
}
