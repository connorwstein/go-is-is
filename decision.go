// TODO: Run SPF on the update db 
// to produce another AVL tree which is the decision db with
// all the best routes
package main


// // Implementation - similar to the other modules, there is a goroutine 
// // which is blocked on an event channel. The events coming on the channel are simple signals 
// // that the SPF database should be recomputed. 
// // This goroutine is not interface specific, rather it is for the entire update db
// func isisDecision(spfEventQueue chan bool){
//     for {
//         spfEvent := <- spfEventQueue
//         fmt.P
//         // Run SPF on the update db
//     }
// }
