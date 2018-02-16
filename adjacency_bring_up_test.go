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

func ConfigureSid(host string, port string, sid string) {
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

func GetState(host string, port string) *pb.StateReply {
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
    return r
}

func TestAdjBringUp(t *testing.T) {
    // Configure SIDs of the two nodes
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    fmt.Println(nodeIpAddresses)
    for k := 0; k < len(nodeIpAddresses); k++ {
        ConfigureSid(nodeIpAddresses[k], "50051", fmt.Sprintf("1111.1111.111%d", k + 1))
    }
    // Poll for adjacency establishment
    adjCount := make(map[int]int) 
    maxPolls := 10
    currPoll := 0
    for currPoll < maxPolls {
        for k := 0; k < len(nodeIpAddresses); k++ {
            intfs := GetState(nodeIpAddresses[k], "50051").Intf
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
}
