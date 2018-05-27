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
	"fmt"
    "flag"
	pb "github.com/connorwstein/go-is-is/config"
	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"os"
	"strings"
	"testing"
	"time"
)


var numNodes = flag.Int("num_nodes", 3, "Number of nodes in the topology")
var serverConnections map[string]*grpc.ClientConn = make(map[string]*grpc.ClientConn)

func CreateClients(host string, port string) (pb.StateClient, pb.ConfigureClient) {
	target := [2]string{host, port}
	conn, err := grpc.Dial(strings.Join(target[:], ":"), grpc.WithInsecure())
	if err != nil {
		glog.Errorf("Failed to connect to gRPC server: %v", err)
	}
    serverConnections[host] = conn
    state := pb.NewStateClient(conn)
    config := pb.NewConfigureClient(conn) 
    return state, config
}

func CloseConnections() {
    for _, v := range serverConnections {
        v.Close()
    }
}

func Get(host string, port string, req string) interface{} {
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

var systemIDtests = []struct {
    in string
    out string
}{{"1111.1111.1111","SID 1111.1111.1111 successfully configured"}, 
  {"1111.1111.1112","SID 1111.1111.1112 successfully configured"},
  {"1111.1111.1113","SID 1111.1111.1113 successfully configured"},
  {"1111.1111.1114","SID 1111.1111.1114 successfully configured"},
  {"1111.1111.1115","SID 1111.1111.1115 successfully configured"},
  {"1111.1111.1116","SID 1111.1111.1116 successfully configured"},
  {"1111.1111.1117","SID 1111.1111.1117 successfully configured"},
  {"1111.1111.1118","SID 1111.1111.1118 successfully configured"}}


func TestSystemIDConfig(t *testing.T) {
    for i := 1; i <= *numNodes; i++ {
        ip := os.Getenv(fmt.Sprintf("node%d", i))
        _, config := CreateClients(ip, GRPC_CFG_SERVER_PORT)
        rsp, err := config.ConfigureSystemID(context.Background(), &pb.SystemIDCfgRequest{Sid: systemIDtests[i].in})
        if err != nil {
            t.Logf("Unable to configure SID: %v", err)
            t.Fail()
        }
        if rsp.Ack != systemIDtests[i].out {
            t.Fail()
        }
    }
    CloseConnections() 
}

func processAdjacencies(intfs *pb.IntfReply) (int, int) {
    numAdjUp, numAdj := 0, 0
    // We don't actually know how many adjacencies each node has apriori, need 
    // to store this 
    for _, intf := range intfs.Intf {
        if strings.Contains(intf, "UP") {
            numAdjUp += 1
        }
        numAdj += 1 
    }
    return numAdjUp, numAdj
}

func checkAdjacenciesReady(t *testing.T) bool {
	adjCount := make(map[string]int)
	desiredAdjCount := make(map[string]int)
    for i := 1; i <= *numNodes; i++ {
        ip := os.Getenv(fmt.Sprintf("node%d", i))
        state, _ := CreateClients(ip, GRPC_CFG_SERVER_PORT)
        intfs, err := state.GetIntf(context.Background(), &pb.IntfRequest{ShIntf: ""})
        if err != nil {
            t.Logf("Error querying interface state %v", err)
            t.Fail()
        }
        // We don't actually know how many adjacencies each node has apriori, need 
        // to store this 
        adjCount[ip], desiredAdjCount[ip] = processAdjacencies(intfs)
    }
    t.Log("Adjacenecy Count:")
    for k, v := range adjCount {
        t.Log(k, v)
    } 
    t.Log("Desired Adjacency Count:")
    for k, v := range desiredAdjCount{
        t.Log(k, v)
    } 
    allValid := true 
    for i := 1; i <= *numNodes; i++ {
        ip := os.Getenv(fmt.Sprintf("node%d", i))
        if adjCount[ip] != desiredAdjCount[ip] {
            allValid = false
        }
    }
    return allValid
}

func TestAdjBringUp(t *testing.T) {
	// Poll for adjacency establishment
    // Can't really use a table test here as 
    // we get the adjacency information on the fly
    timeout := time.After(10*time.Second)
    ticker := time.Tick(500*time.Millisecond) 
    for {
        select {
        case <- timeout:
            t.Log("Adjacencies did not come up in time")
            t.Fail()
        case <-ticker:
            t.Log("Check adjacencies")
            if checkAdjacenciesReady(t) {
                CloseConnections() 
                return  
            }
        }
    }
    CloseConnections() 
}


// func TestLspFlooding(t *testing.T) {
// 	nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
// 	time.Sleep(10000 * time.Millisecond) // Give it some time for LSP flooding
// 	// Ensure UpdateDB has been replicated on each node
// 	// Expecting to see three lsps 1111, 1112, 1113
// 	lspCheck := make(map[string]bool)
// 	lspCheck["1111.1111.1111"] = false
// 	lspCheck["1111.1111.1112"] = false
// 	lspCheck["1111.1111.1113"] = false
// 	for k := 0; k < len(nodeIpAddresses); k++ {
// 		tmp := Get(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, "lsp")
// 		lsps := tmp.(*pb.LspReply).Lsp
// 		for _, lsp := range lsps {
// 			glog.V(1).Infof("%v\n", lsp[:14])
// 			if _, ok := lspCheck[lsp[:14]]; !ok {
// 				// Unknown lsp
// 				glog.V(1).Infof("Unexpected LSP in database %v", lsp[:14])
// 				t.Fail()
// 			} else {
// 				lspCheck[lsp[:14]] = true
// 			}
// 		}
// 		// Confirm that all expected lsps are present for this node, then reset to false
// 		for lsp, present := range lspCheck {
// 			if !present {
// 				glog.V(1).Infof("Failed to find LSP %v in database on node %v", lsp[:14], nodeIpAddresses[k])
// 				t.Fail()
// 			} else {
// 				// Reset it for the next node
// 				lspCheck[lsp] = false
// 			}
// 		}
// 	}
// }
// 
// func TestTopo(t *testing.T) {
// 	//     setDebugs("3")
// 	nodeIpAddresses := []string{os.Getenv("node1"), os.Getenv("node2"), os.Getenv("node3")}
// 	// Just a sanity check to make sure the topo contains all nodes
// 	// More detailed tests in the decision test cases
// 	for k := 0; k < len(nodeIpAddresses); k++ {
// 		tmp := Get(nodeIpAddresses[k], GRPC_CFG_SERVER_PORT, "topo")
// 		topos := tmp.(*pb.TopoReply).Topo
// 		// + 1 is because we get the LSP ID + the # number of nodes in the topology
// 		if len(topos) != (len(nodeIpAddresses) + 1) {
// 			glog.V(3).Infof("Expecting %v topos, got %v", len(nodeIpAddresses)+1, len(topos))
// 			t.Fail()
// 		}
// 		for _, topo := range topos[1:] {
// 			glog.V(3).Infof("Topo %v", topo)
// 		}
// 	}
// }
