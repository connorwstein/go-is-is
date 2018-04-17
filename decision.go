// Decision process in the IS-IS protocol.
// Computes the shortest paths to all other nodes based on the update database information.
package main

import (
    "sync"
    "fmt"
    "github.com/golang/glog"
)

var TopoDB *IsisDB

type Triple struct {
    // Either systemID or prefix is set, not both
    systemID string
    distance uint32
    adj *Adjacency
}

func (t Triple) String() string {
    if t.adj == nil {
        return fmt.Sprintf("SystemID %s Distance %d Next Hop %v", t.systemID, t.distance, t.adj)
    } else {
        return fmt.Sprintf("SystemID %s Distance %d Next Hop %s Intf %s", t.systemID, t.distance, systemIDToString(t.adj.neighborSystemID), t.adj.intfName)
    }
}

func topoDBInit() {
    TopoDB = &IsisDB{DBLock: sync.Mutex{}, Root: nil}
}

func isisDecision(triggerSPF chan bool){
    // Implementation - similar to the other modules, there is a goroutine 
    // which is blocked on an event channel. The events coming on the channel are simple signals 
    // that the SPF database should be recomputed. Maybe if we want to get fancy they could indicate whether you 
    // only need to do a partial recompute.
    // This goroutine is not interface specific, rather it is for the entire update db
    for {
        glog.V(2).Infof("SPF: Waiting for SPF event")
        spf := <- triggerSPF
        glog.V(2).Infof("SPF: Received trigger")
        // Run SPF on the update db to build the network topology
        if spf {
            glog.V(2).Infof("SPF: Compute SPF")
            computeSPF(UpdateDB, TopoDB, cfg.sid, cfg.interfaces)
        }
    }
}

func printPaths(prefix string, paths []*Triple) {
    for _, path := range paths {
        glog.V(2).Infof("%v", path)
    }
}

func getAdjacencies(source *Triple, unknown []*AvlNode) []*Triple {
    // Given a source triple return a slice of triples from unknown
    // Update the costs appropriately based on the cost to source
    trips := make([]*Triple, 0)
    for _, node := range unknown {
        lsp := node.data.(*IsisLsp)
        if source.systemID == systemIDToString(lsp.LspID[:6]) {
            glog.V(2).Infof("SPF: Found our systemID %s", source.systemID)
            // Found our lsp, grabs its neighbors
            neighbors := lookupNeighbors(lsp)
            for _, neighbor := range neighbors {
                glog.V(2).Infof("SPF: Neighbor %s", neighbor.systemID)
                // Given a neighbor system id find the adjacency
                trips = append(trips, &Triple{systemID: neighbor.systemID, distance: neighbor.metric, adj: getAdjacency(neighbor.systemID)})
            }
        }
    }
    return trips
}

func checkInPaths(adj *Triple, paths []*Triple) bool {
    for _, path := range paths {
        if adj.systemID == path.systemID {
            return true 
        }
    }
    return false
}

func addAdjToTent(currentDistance uint32, currentAdj *Adjacency, neighbor *Triple, tent *[]*Triple, paths []*Triple) int {
    // Returns 0 if just an update, or 1 if added
    // Ignore if already in paths
    // Update distance if already present
    // otherwise add
    if checkInPaths(neighbor, paths) {
        return 0
    }
    found := false
    for i, system := range *tent {
        if neighbor.systemID == system.systemID {
            found = true
            // Already there, update distance if we can
            if (neighbor.distance + currentDistance) < system.distance {
                (*tent)[i].distance = neighbor.distance + currentDistance
            }
        }
    }
    if ! found {
        glog.V(2).Infof("Adding system %s", neighbor.systemID)
        neighbor.distance += currentDistance
        neighbor.adj = currentAdj
        *tent = append(*tent, neighbor)
        return 1
    }
    return 0
}

func computeSPF(updateDB *IsisDB, topoDB *IsisDB, localSystemID string, localInterfaces []*Intf) {
    // db.Root is an AVL tree where the nodes contain LSPs
    // Compute the shortest paths to all the prefixes found in the tree 
    // All prefixes will be leaves
    // Metric information (distance) and neighbor relationships are contained in the LSP's TLVs
    // Probably some way to optimize this by not taking both locks
    // Update the decision DB
    // The local systemID is our starting point for dijkstra
    glog.V(2).Info("SPF: Running SPF, taking update database lock")
    updateDB.DBLock.Lock()
    // SPF time
    // Decision DB should still be keyed by system ID, but its contents should be prefixes and their associated costs and next hops?
    // For the first crack at this let assume there are no-parallel edges (i.e. two adjacencies to the same next hop)
    // We need 2 lists: paths and tent
    // Each of the form < string systemID, uint32 metric, *Adjacency>
    // Don't need to touch the UpdateDB for the first iteration, just use our adjacency database directly
    paths := make([]*Triple, 0)
    tent := make([]*Triple, 0)
    // TODO: optimize this
    unknown := AvlGetAll(updateDB.Root)
    localSystemIDIndex := -1
    for i, node := range unknown {
        if systemIDToString(node.data.(*IsisLsp).LspID[:6]) == localSystemID {
            paths = append(paths, &Triple{systemID: localSystemID}) // Leave distance 0 and adj nil
            localSystemIDIndex = i
        }
    }
    if localSystemIDIndex == -1 {
        glog.Errorf("Unable to find our own lsp, cannot compute SPF")
        updateDB.DBLock.Unlock()
        return
    }
    // Yeah, yeah this is slow. Remove our own lsp
    unknown = append(unknown[:localSystemIDIndex], unknown[localSystemIDIndex + 1:]...)
    
    // Load tent with our local adjacencies and directly connected prefixes
    // How to handle directly connected prefixes ?
    // The system id in the triple can also be a prefix. In real IS-IS however, this would only happen in a L2 router.
    for _, intf := range localInterfaces {
        if intf.adj.state == "UP" {
            tent = append(tent, &Triple{systemID: systemIDToString(intf.adj.neighborSystemID), distance: intf.adj.metric, adj: intf.adj})
        }
    } 
    // At each step of the algorithm, the TENT list is examined, and the node with the least cost from the source is moved into PATHS. 
    // When a node is placed in PATHS, all IP prefixes advertised by it are installed in the IS-IS Routing Information Base (RIB) 
    // with the corresponding metric and next hop. The directly connected neighbors of the node that just made it into PATHS are then added 
    // to TENT if they are not already there and their associated costs adjusted accordingly, for the next selection.
    tentSize := len(tent)
    printPaths("path", paths)
    for tentSize > 0 {
        // Examine each element in tent, looking for the shortest path from ourselves to that node/prefix
        // After finding the shortest one, update the metric appopriately and add it to paths. Then add all adjacencies for that guy to tent.
        // If the paths are equal, pick one arbitrarily?
        glog.V(2).Infof("SPF: tent size %d len tent %d", tentSize, len(tent))
        printPaths("tent", tent)
        minCostFromSource := ^uint32(0) // Max uint
        bestPathIndex := -1
        for i, candidate := range tent {
            if candidate.distance < minCostFromSource {
                glog.V(2).Infof("SPF: Best system %v", candidate)
                minCostFromSource = candidate.distance
                bestPathIndex = i
            }
        }
        glog.V(2).Infof("SPF: Best candidate %s, cost %d", tent[bestPathIndex].systemID, minCostFromSource)
        // We now have the best candidate
        // Add it to paths
        paths = append(paths, tent[bestPathIndex])
        // Save this as the next hop for its neighbors
        currentNextHop := tent[bestPathIndex].adj
        // Add all of this new guys adjacencies, some triple, need to find all of its adjacencies
        bestAdjacencies := getAdjacencies(tent[bestPathIndex], unknown)
        // Remove from tents
        tent = append(tent[:bestPathIndex], tent[bestPathIndex + 1:]...)
        glog.V(2).Infof("SPF: tent size %d len tent %d", tentSize, len(tent))
        tentSize--
        // Now add all those adjacencies to tent
        // If the node is already in tent but it has a longer path, then update the distance
        for _, adj := range bestAdjacencies {
            added := addAdjToTent(minCostFromSource, currentNextHop, adj, &tent, paths)
            if added == 1 {
                glog.V(2).Infof("SPF: added adj: %s", adj.systemID)
            } else {
                glog.V(2).Infof("SPF: did not add adj: %s", adj.systemID)
            }
            tentSize += added
            glog.V(2).Infof("SPF: tent size %d len tent %d", tentSize, len(tent))
        } 
    } 
    printPaths("path", paths)
    for _, path := range paths {
        topoDB.Root = AvlInsert(topoDB.Root, systemIDToKey(path.systemID), path, true)
    }    
    AvlPrint(topoDB.Root)
    updateDB.DBLock.Unlock()
}
