// Steps in this test:
// 1. Configure the SIDs of the two nodes via gRPC client
// 2. Confirm the SIDs are correctly configured 
// 3. Ensure that the adjacency has been established
// The tests are run from a separate container
// This is an example client which can be used to configure the IS-IS node
// Will need the ip addresses of the other two containers
package main

import (
    "log"
    "os"
    "fmt"
    "time"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    pb "../config"
    "strings"
)

func configure_sid(host string, port string, sid string) {
    target := [2]string{host, port}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewConfigureClient(conn)
    r, err := c.ConfigureSystemID(context.Background(), &pb.SystemIDRequest{Sid: sid})
    if err != nil {
        log.Fatalf("Unable to configure SID: %v", err)
    }
    log.Printf("SID configure result: %s", r.Message)
}

func get_state(host string, port string) {
    // TODO: reuse this connection
    target := [2]string{host, port}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewStateClient(conn)
    
    r, err := c.GetState(context.Background(), &pb.StateRequest{ShRun: ""})
    if err != nil {
        log.Fatalf("Unable to get state: %v", err)
    }
    log.Printf("State response %s", r)
}

func main() {
    // Configure SIDs of the two nodes
    node_ip_addresses := os.Args[1:]
    fmt.Println(node_ip_addresses)
//     for k := 0; k < len(node_ip_addresses); k++ {
//         configure_sid(node_ip_addresses[k], "50051", fmt.Sprintf("1111.1111.111%d", k + 1))
//     }
    // Poll for adjacency establishment
    for {
        for k := 0; k < len(node_ip_addresses); k++ {
             get_state(node_ip_addresses[k], "50051")
        }
        time.Sleep(5000 * time.Millisecond)
   }
}
