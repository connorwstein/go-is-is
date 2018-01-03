// This is an example client which can be used to configure the IS-IS node
package main

import (
    "log"
    "os"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    pb "../config"
    "strings"
)


func main() {
    if len(os.Args) != 3 {
        log.Fatalf("Usage: go run test_client/main.go <host> <port>")
    }
    conn, err := grpc.Dial(strings.Join(os.Args[1:], ":"), grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewConfigureClient(conn)
    r, err := c.ConfigureSystemID(context.Background(), &pb.SystemIDRequest{Sid: "1111.1111.1111"})
    if err != nil {
        log.Fatalf("Unable to configure SID: %v", err)
    }
    log.Printf("SID configure result: %s", r.Message)
}
