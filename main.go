package main

import (
    "fmt"
    "time"
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
)

var wg sync.WaitGroup
var l1_lan_hello_dst []byte
var cfg Config

// gRPC server port, listening for incoming configuration
const (
    port = ":50051"
)

type Config struct {
    sid string
}

type Adjacency struct {
    intf string
    state string // Can be NEW, INITIALIZING or UP
}

func hello_send() {
    // Send hellos every 2 seconds after a system ID has been configured
    for {
        send_frame(l1_lan_hello_dst, "eth0")
        // After sending we update the adjacency to NEW
        time.Sleep(2000 * time.Millisecond)
    }
}

func hello_recv() {
    for {
        // Blocks until a hello pdu is received
        hello := recv_frame("eth0")
        fmt.Printf("Got hello from %X:%X:%X:%X:%X:%X\n", 
                   hello[6], hello[7], hello[8], hello[9], hello[10], hello[11])
        fmt.Println(hex.Dump(hello[14:]))
        // Depending on what type of hello it is, respond
        // Respond to this hello packet with a IS-Neighbor TLV 
    }
}

func cleanup() {
    fmt.Println("cleanup")
}

type server struct{}

func (s *server) ConfigureSystemID(ctx context.Context, in *pb.SystemIDRequest) (*pb.SystemIDReply, error) {
    fmt.Println("Got SID request")
    // Returning a pointer to the system ID reply struct with a message acknowledging that it was 
    // successfully configured.
    // Note that even through the proto has a the field defined with lowercase, it is converted
    // to uppercase so it can be exported golang style
    return &pb.SystemIDReply{Message: "SID " + in.Sid + " successfully configured"}, nil
}

func start_grpc() {
    lis, err := net.Listen("tcp", port)
    if err != nil {
        log.Fatalf("gRPC server failed to start listening: %v", err)
    }
    s := grpc.NewServer()
    pb.RegisterConfigureServer(s, &server{})
    // Register reflection service on gRPC server.
    reflection.Register(s)
    if err := s.Serve(lis); err != nil {
        log.Fatalf("gRPC server failed to start serving: %v", err)
    }
}
func main() {
    // This is a special multicast mac address
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
//     wg.Add(1)
//     go hello_send()
//     wg.Add(1)
//     go hello_recv()
    wg.Add(1)
    go start_grpc()
    wg.Wait()
}
