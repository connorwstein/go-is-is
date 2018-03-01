package main

import (
    "flag"
    "strings"
    "fmt"
//     "time"
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
	"reflect"
    "github.com/golang/glog"
    "runtime"
    "strconv"
)

var wg sync.WaitGroup
var l1_multicast []byte
var cfg Config

const (
    GRPC_CFG_SERVER_PORT = ":50051"
    RECV_LOG_PREFIX = "RECV:"
    SEND_LOG_PREFIX = "SEND:"
    CHAN_BUF_SIZE = 100
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
}

type Adjacency struct {
    state string // Can be NEW, INITIALIZING or UP
    neighbor_system_id []byte 
}

func system_id_to_str(system_id []byte) string {
    // Byte slice should be 6 bytes
    if len(system_id) != 6 {
        return "" 
    }
    result := ""
    for i := 0; i < 3; i++ {
        result += hex.EncodeToString(system_id[i*2:i*2 + 2])
        if i != 2 {
            result += "."
        } 
    }
    return result
}

func system_id_to_bytes(sid string) [6]byte {
    sid = strings.Replace(sid, ".", "", 6)
    var sidBytes []byte = make([]byte, 6, 6)
    sidBytes, _ = hex.DecodeString(sid)
    var fixed [6]byte
    copy(fixed[:], sidBytes)
    return fixed
}

func getGID() uint64 {
    b := make([]byte, 64)
    b = b[:runtime.Stack(b, false)]
    b = bytes.TrimPrefix(b, []byte("goroutine "))
    b = b[:bytes.IndexByte(b, ' ')]
    n, _ := strconv.ParseUint(string(b), 10, 64)
    return n
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
    var reply pb.StateReply
    reply.Intf = make([]string, len(cfg.interfaces))
    for i, intf:= range cfg.interfaces {
        interfaces_string := ""
        if intf.adj.state != "UP" {
            interfaces_string += intf.prefix.String() + " " + intf.mask.String() + ", adjacency " + intf.adj.state
        } else {
            interfaces_string += intf.prefix.String() + " " + intf.mask.String() + ", adjacency " + intf.adj.state + " with " + system_id_to_str(intf.adj.neighbor_system_id)
        }
        fmt.Println(reflect.TypeOf(reply.Intf))
        reply.Intf[i] = interfaces_string
    }
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
    l1_multicast = []byte{0x01, 0x80, 0xc2, 0x00, 0x00, 0x14}
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
    ethernetInit()

    for _, intf := range cfg.interfaces {
        ethernetIntfInit(intf.name) // Creates send/recv raw sockets
    }
	// Start a couple go routines to communicate with other nodes
	// to establish adjacencies. Each go routine can run
    // totally in parallel to establish adjacencies on each
    // interface
    // Each goroutine blocks on the hello channel waiting for a hello pdu
    // from the recvPdus goroutine
    wg.Add(1) // Just need one of these because none of the goroutines should exit
    var helloChans, updateChans []chan [READ_BUF_SIZE]byte
    var sendChans []chan []byte
    for i := 0; i < len(cfg.interfaces); i++ {
        helloChans = append(helloChans, make(chan [READ_BUF_SIZE]byte, CHAN_BUF_SIZE))
        updateChans = append(updateChans, make(chan [READ_BUF_SIZE]byte, CHAN_BUF_SIZE))
        sendChans = append(sendChans, make(chan []byte, CHAN_BUF_SIZE))
    }
    for i, intf := range cfg.interfaces {
        go isisHelloSend(intf, sendChans[i])
        go isisHelloRecv(intf, helloChans[i], sendChans[i])
        go recvPdus(intf.name, helloChans[i], updateChans[i])
        go sendPdus(intf.name, sendChans[i])
        go isisUpdate(sendChans[i])
        go isisUpdateInput(intf, updateChans[i], sendChans[i])
    }
    // Start the gRPC server for accepting configuration (CLI commands)
    go start_grpc()
    wg.Wait()
}
