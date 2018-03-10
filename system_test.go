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
    pb "./config"
    "strings"
    "testing"
)

func ConfigureSid(host string, port string, sid string) *pb.SystemIDCfgReply {
    target := [2]string{host, port}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewConfigureClient(conn)
    rsp, err := c.ConfigureSystemID(context.Background(), &pb.SystemIDCfgRequest{Sid: sid})
    if err != nil {
        log.Fatalf("Unable to configure SID: %v", err)
    }
    return rsp
}

func Get(host string, port string, req string) interface{} {
    // TODO: reuse this connection
    target := [2]string{host, port}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewStateClient(conn)
     
    if req == "intf" {
        r, err := c.GetIntf(context.Background(), &pb.IntfRequest{ShIntf: ""})
        if err != nil {
            log.Fatalf("Unable to get state: %v", err)
        }
        log.Printf("Intf response %s", r)
        return r
    } else if req == "lsp" {
        r, err := c.GetLsp(context.Background(), &pb.LspRequest{ShLsp: ""})
        if err != nil {
            log.Fatalf("Unable to get state: %v", err)
        }
        log.Printf("Intf response %s", r)
        return r
    } 
    return nil
}

func TestSystemIDConfig(t *testing.T) {
    // Configure SIDs of the three nodes
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    for k := 0; k < len(nodeIpAddresses); k++ {
        rsp := ConfigureSid(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, fmt.Sprintf("1111.1111.111%d", k + 1))
        fmt.Println(rsp.Ack)
        if ! strings.Contains(rsp.Ack, "successfully") {
            t.Fail()
        }
    }
}


func TestAdjBringUp(t *testing.T) {
    // Poll for adjacency establishment
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    adjCount := make(map[int]int) 
    maxPolls := 10
    currPoll := 0
    for currPoll < maxPolls {
        for k := 0; k < len(nodeIpAddresses); k++ {
            tmp := Get(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, "intf")
            intfs := tmp.(*pb.IntfReply).Intf
            numAdjUp := 0  
            for _, intf := range intfs{
                if strings.Contains(intf, "UP") {
                    numAdjUp += 1
                }
            }
            adjCount[k] = numAdjUp
        }
        if adjCount[0] == 1 && adjCount[1] == 2 && adjCount[2] == 1 {
            // All desired adjacencies are up
            break
        }
        currPoll += 1
        time.Sleep(2000 * time.Millisecond)
    }
    if currPoll == maxPolls {
        t.Fail()
    }
}

func TestLspFlooding(t *testing.T) {
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    time.Sleep(7000 * time.Millisecond)  // Give it some time for LSP flooding
    // Once all the adjacencies are up, print the LSPs
    for k := 0; k < len(nodeIpAddresses); k++ {
        tmp := Get(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, "lsp")
        lsps := tmp.(*pb.LspReply).Lsp
        for _, lsp := range lsps {
            log.Printf("LSP %v", lsp)
        }
    }
}
