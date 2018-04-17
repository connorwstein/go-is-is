// Simple script to dump the LSP DB on a node 
package main

import (
    "fmt"
    "os"
    pb "../config"
    "google.golang.org/grpc"
    "strings"
    "golang.org/x/net/context"
)

func main() {
    target := [2]string{os.Args[1], "50051"}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        fmt.Printf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewStateClient(conn)
    showSystemID, err := c.GetSystemID(context.Background(), &pb.SystemIDRequest{ShSystemID: ""})
    if err != nil {
        fmt.Printf("Unable to get state: %v", err)
    }
    fmt.Println("System ID:", showSystemID.Sid) 
    showIntf, err := c.GetIntf(context.Background(), &pb.IntfRequest{ShIntf: ""})
    if err != nil {
        fmt.Printf("Unable to get state: %v", err)
    }
    for _, intf := range showIntf.Intf {
        fmt.Println("ADJ:", intf)
    }

    fmt.Println("LSP DB:", os.Args[1])
    showLsp, err := c.GetLsp(context.Background(), &pb.LspRequest{ShLsp: ""})
    if err != nil {
        fmt.Printf("Unable to get state: %v", err)
    }
    for _, lsp := range showLsp.Lsp {
        fmt.Println("LSP:", lsp)
    }

    fmt.Println("TOPO:")
    showTopo, err := c.GetTopo(context.Background(), &pb.TopoRequest{ShTopo: ""})
    if err != nil {
        fmt.Printf("Unable to get state: %v", err)
    }
    for _, topo := range showTopo.Topo {
        fmt.Println("LSP:", topo)
    }
}
