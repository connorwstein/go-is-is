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
//    "os"
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

func main() {
    // Configure SIDs of the two nodes
    // Because the test node is the last node in the docker compose file it will have the ip address 172.18.0.4
    configure_sid("172.18.0.2", "50051", "1111.1111.1111")
    configure_sid("172.18.0.3", "50051", "1111.1111.1112")
}
