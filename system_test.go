// Steps in this test:
// 1. Configure the SIDs of the two nodes via gRPC client
// 2. Confirm the SIDs are correctly configured 
// 3. Ensure that the adjacency has been established
// The tests are run from a separate container
// This is an example client which can be used to configure the IS-IS node
// Will need the ip addresses of the other two containers
// TODO: Given a topology which is a connected graph, make this system test generic enough to handle it
package main

import (
    "os"
    "fmt"
    "time"
    "golang.org/x/net/context"
    "google.golang.org/grpc"
    "github.com/golang/glog"
    pb "./config"
    "strings"
    "testing"
)

func ConfigureSid(host string, port string, sid string) *pb.SystemIDCfgReply {
    target := [2]string{host, port}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        glog.Errorf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewConfigureClient(conn)
    rsp, err := c.ConfigureSystemID(context.Background(), &pb.SystemIDCfgRequest{Sid: sid})
    if err != nil {
        glog.Errorf("Unable to configure SID: %v", err)
    }
    return rsp
}

func Get(host string, port string, req string) interface{} {
    // TODO: reuse this connection
    target := [2]string{host, port}
    conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
    if err != nil {
        glog.Errorf("Failed to connect to gRPC server: %v", err)
    }
    defer conn.Close()

    c := pb.NewStateClient(conn)
     
    if req == "intf" {
        r, err := c.GetIntf(context.Background(), &pb.IntfRequest{ShIntf: ""})
        if err != nil {
            glog.Errorf("Unable to get state: %v", err)
        }
        return r
    } else if req == "lsp" {
        r, err := c.GetLsp(context.Background(), &pb.LspRequest{ShLsp: ""})
        if err != nil {
            glog.Errorf("Unable to get state: %v", err)
        }
        glog.V(3).Infof("Lsp response %s", r)
        return r
    } else if req == "topo" {
        r, err := c.GetTopo(context.Background(), &pb.TopoRequest{ShTopo: ""})
        if err != nil {
            glog.Errorf("Unable to get state: %v", err)
        }
        glog.V(3).Infof("Topo response %s", r)
        return r
    } 
    return nil
}

func TestSystemIDConfig(t *testing.T) {
    // Configure SIDs of the three nodes
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    for k := 0; k < len(nodeIpAddresses); k++ {
        rsp := ConfigureSid(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, fmt.Sprintf("1111.1111.111%d", k + 1))
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
//     setDebugs("3")
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    time.Sleep(10000 * time.Millisecond)  // Give it some time for LSP flooding
    // Ensure UpdateDB has been replicated on each node
    // Expecting to see three lsps 1111, 1112, 1113
    lspCheck := make(map[string]bool)
    lspCheck["1111.1111.1111"] = false
    lspCheck["1111.1111.1112"] = false
    lspCheck["1111.1111.1113"] = false
    for k := 0; k < len(nodeIpAddresses); k++ {
        tmp := Get(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, "lsp")
        lsps := tmp.(*pb.LspReply).Lsp
        for _, lsp := range lsps {
            glog.V(1).Infof("%v\n", lsp[:14])
            if _, ok := lspCheck[lsp[:14]]; ! ok {
                // Unknown lsp
                glog.V(1).Infof("Unexpected LSP in database %v", lsp[:14])
                t.Fail()
            } else {
                lspCheck[lsp[:14]] = true
            }
        }
        // Confirm that all expected lsps are present for this node, then reset to false
        for lsp, present := range lspCheck {
            if ! present {
                glog.V(1).Infof("Failed to find LSP %v in database on node %v", lsp[:14], nodeIpAddresses[k])
                t.Fail()
            } else {
                // Reset it for the next node
                lspCheck[lsp] = false
            }
        } 
    }
//     setDebugs("0")
}

func TestTopo(t *testing.T) {
//     setDebugs("3")
    nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
    // Just a sanity check to make sure the topo contains all nodes
    // More detailed tests in the decision test cases
    for k := 0; k < len(nodeIpAddresses); k++ {
        tmp := Get(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, "topo")
        topos := tmp.(*pb.TopoReply).Topo
        // + 1 is because we get the LSP ID + the # number of nodes in the topology
        if len(topos) != (len(nodeIpAddresses) + 1){
            glog.V(3).Infof("Expecting %v topos, got %v", len(nodeIpAddresses) + 1, len(topos))
            t.Fail()
        }
        for _, topo := range topos[1:] {
            glog.V(3).Infof("Topo %v", topo)
        }
    }
}
