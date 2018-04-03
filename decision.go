// TODO: Run SPF on the update db 
// to produce another AVL tree which is the decision db with
// all the best routes
package main

import (
    "sync"
    "net"
    "github.com/golang/glog"
)

var DecisionDB *IsisDB

func DecisionDBInit() {
    DecisionDB = &IsisDB{DBLock: sync.Mutex{}, Root: nil}
}

// Implementation - similar to the other modules, there is a goroutine 
// which is blocked on an event channel. The events coming on the channel are simple signals 
// that the SPF database should be recomputed. Maybe if we want to get fancy they could indicate whether you 
// only need to do a partial recompute.
// This goroutine is not interface specific, rather it is for the entire update db
func isisDecision(spfEventQueue chan bool){
    for {
//         spfEvent := <- spfEventQueue
//         // Run SPF on the update db
//         // and update the decision db
//         computeSPF(UpdateDB, DecisionDB, cfg.sid)
    }
}

type Triple struct {
    // Either systemID or prefix is set, not both
    systemID string
    prefix *net.IPNet
    distance uint32
    adj *Adjacency
}

func computeSPF(updateDB *IsisDB, decisionDB *IsisDB, localSystemID string, localInterfaces []*Intf) {
    // db.Root is an AVL tree where the nodes contain LSPs
    // Compute the shortest paths to all the prefixes found in the tree 
    // All prefixes will be leaves
    // Metric information (distance) and neighbor relationships are contained in the LSP's TLVs
    // Probably some way to optimize this by not taking both locks
    // Update the decision DB
    // The local systemID is our starting point for dijkstra
    updateDB.DBLock.Lock()
//     decisionDB.DBLock.Lock()
    // SPF time
    // Decision DB should still be keyed by system ID, but its contents should be prefixes and their associated costs and next hops?
    // For the first crack at this let assume there are no-parallel edges (i.e. two adjacencies to the same next hop)
    // We need 2 lists: paths and tent
    // Each of the form < string systemID, uint32 metric, *Adjacency>
    // Don't need to touch the UpdateDB for the first iteration, just use our adjacency database directly
    paths := make([]*Triple, 0)
    tent := make([]*Triple, 0)
    // TODO: optimize this
    unknown := GetAllLsps(updateDB)
    localSystemIDIndex := -1
    for i, lsp := range unknown {
        glog.V(2).Infof("Unknown LSP %v", lsp)
        if system_id_to_str(lsp.LspID[:6]) == localSystemID {
            glog.V(2).Infof("Adding LSP %v to paths", lsp)
            paths = append(paths, &Triple{systemID: localSystemID}) // Leave distance 0 and adj nil
            localSystemIDIndex = i
        }
    }
    glog.V(2).Infof("SPF: paths: %v", paths)
    // Yeah, yeah this is slow. Remove our own lsp
    unknown = append(unknown[:localSystemIDIndex], unknown[localSystemIDIndex + 1:]...)
    
    // Load tent with our local adjacencies and directly connected prefixes
    // How to handle directly connected prefixes ?
    // The system id in the triple can also be a prefix. In real IS-IS however, this would only happen in a L2 router.
    for _, intf := range localInterfaces {
        if intf.adj.state == "UP" {
            tent = append(tent, &Triple{systemID: system_id_to_str(intf.adj.neighbor_system_id), distance: intf.adj.metric, adj: intf.adj})
            for _, route := range intf.routes {
                // TODO: store this metric with the prefix
                tent = append(tent, &Triple{prefix: route, distance: 10}) 
            }
        }
    } 
/*
At each step of the algorithm, the TENT list is examined, and the node with the least cost from the source is moved into PATHS. When a node is placed in PATHS, all IP prefixes advertised by it are installed in the IS-IS Routing Information Base (RIB) with the corresponding metric and next hop. The directly connected neighbors of the node that just made it into PATHS are then added to TENT if they are not already there and their associated costs adjusted accordingly, for the next selection.
*/
    tentSize := len(tent)
    glog.V(2).Infof("SPF: paths: %v", paths)
    for tentSize != 0 {
        // Examine each element in tent, looking for the shortest path from ourselves to that node/prefix
        // After finding the shortest one, update the metric appopriately and add it to paths. Then add all adjacencies for that guy to tent.
        // If the paths are equal, pick one arbitrarily?
        glog.V(2).Infof("SPF: tent: %v", tent)
        minCostFromSource := ^uint32(0) // Max uint
        bestPathIndex := -1
        for i, candidate := range tent {
            if candidate.prefix != nil {
                glog.V(2).Infof("SPF: Examining prefix %v", candidate)
            } else {
                glog.V(2).Infof("SPF: Examining system %v", candidate)
            }
            if candidate.distance < minCostFromSource {
                minCostFromSource = candidate.distance
                bestPathIndex = i
            }
        }
        // We now have the best candidate
        // Add it to paths
        paths = append(paths, tent[bestPathIndex])
        tent = append(tent[:bestPathIndex], tent[bestPathIndex + 1:]...)
        tentSize--
    } 
    glog.V(2).Infof("SPF: paths: %v", paths)
//     decisionDB.DBLock.Unlock()
    updateDB.DBLock.Unlock()
}

// func PrintDecisionDB() {
//     // TODO: Combine this with the update DB one, passing in a walk function
//     if root != nil {
//         PrintDecisionDB(root.left)
//         if root.data != nil {
//             lsp := root.data.(*IsisLsp)
//             glog.Infof("%s", system_id_to_str(lsp.LspID[:6]));
//             glog.V(1).Infof("%s -> %v", system_id_to_str(lsp.LspID[:6]), lsp);
//             var curr *IsisTLV = lsp.CoreLsp.FirstTlv
//             for curr != nil {
//                 glog.V(1).Infof("\tTLV %d\n", curr.tlv_type);
//                 glog.V(1).Infof("\tTLV size %d\n", curr.tlv_length);
//                 if curr.tlv_type == 128 {
//                     // This is a external reachability tlv
//                     // TODO: fix hard coding here
//                     for i := 0; i < int(curr.tlv_length) / 12; i++ {
//                         var prefix net.IPNet 
//                         prefix.IP = curr.tlv_value[i*12:i*12 + 4]
//                         prefix.Mask = curr.tlv_value[i*12 + 4: i*12 + 8]
//                         metric := curr.tlv_value[i*12 + 8: i*12 + 12]
//                         glog.V(1).Infof("\t\t%s Metric %d\n", prefix.String(), binary.BigEndian.Uint32(metric[:]));
//                     }
//                 } else if curr.tlv_type == 2 {
//                     // This is a neighbors tlv, its length - 1 (to exclude the first virtualByteFlag) will be a multiple of 11
//                     for i := 0; i < int(curr.tlv_length - 1)  / 11; i++ {
//                         // Print out the neighbor system ids and metric 
//                         metric := curr.tlv_value[i*11 + 4]
//                         systemID := curr.tlv_value[(i*11 + 1 + 4):(i*11 + 1 + 4 + 6)]
//                         glog.V(1).Infof("\t\t%s Metric %d\n", system_id_to_str(systemID), metric)
//                     }
//                 }
//                 curr = curr.next_tlv
//             }            
//         }
//         PrintUpdateDB(root.right)
//     }
// }
