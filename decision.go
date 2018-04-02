// TODO: Run SPF on the update db 
// to produce another AVL tree which is the decision db with
// all the best routes
package main

import (
    "sync"
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

func computeSPF(updateDB *IsisDB, decisionDB *IsisDB, localSystemID string) {
    // db.Root is an AVL tree where the nodes contain LSPs
    // Compute the shortest paths to all the prefixes found in the tree 
    // All prefixes will be leaves
    // Metric information (distance) and neighbor relationships are contained in the LSP's TLVs
    // Probably some way to optimize this by not taking both locks
    // Update the decision DB
    // The local systemID is our starting point for dijkstra
    updateDB.DBLock.Lock()
    decisionDB.DBLock.Lock()
    // SPF time
    decisionDB.DBLock.Unlock()
    updateDB.DBLock.Unlock()
}
